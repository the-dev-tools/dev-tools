// Package rcloudlink implements CloudLinkService: Files rows that mirror Stresseur Cloud repos and
// their flow files, so flows from a repo can be opened, edited and run in Studio and saved back.
package rcloudlink

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rfile"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rflowv2"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcloudlink"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scloudlink"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
	cloudlinkv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/cloud_link/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/cloud_link/v1/cloud_linkv1connect"
)

// ErrDirty is the FAILED_PRECONDITION message of an Upsert over unsaved edits.
const ErrDirty = "local copy has unsaved edits"

var (
	errLinkNotFound      = errors.New("cloud link not found")
	errWorkspaceNotFound = errors.New("workspace not found")
)

// CloudLinkRPCReaders are the read-only services the handler fetches with.
type CloudLinkRPCReaders struct {
	CloudLink *scloudlink.Reader
	File      *sfile.Reader
	Flow      *sflow.FlowReader
}

// CloudLinkRPCStreamers are the sync streams for rows the import creates directly.
type CloudLinkRPCStreamers struct {
	File eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
	Flow eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]
}

// CloudLinkRPCDeps are the handler's dependencies.
type CloudLinkRPCDeps struct {
	DB        *sql.DB
	Queries   *gen.Queries
	User      suser.UserService
	Readers   CloudLinkRPCReaders
	Streamers CloudLinkRPCStreamers
	// Publisher publishes the flow and HTTP rows an import creates (the flow and HTTP services'
	// mutation publishers).
	Publisher mutation.Publisher
	// DeletePublisher publishes the rows a replace or unlink removes (the cascade registry).
	DeletePublisher mutation.Publisher
	Logger          *slog.Logger
}

// Validate checks the required dependencies.
func (d *CloudLinkRPCDeps) Validate() error {
	switch {
	case d.DB == nil:
		return errors.New("db is required")
	case d.Queries == nil:
		return errors.New("queries is required")
	case d.Readers.CloudLink == nil, d.Readers.File == nil, d.Readers.Flow == nil:
		return errors.New("cloud link, file and flow readers are required")
	}
	return nil
}

// CloudLinkRPC implements cloud_linkv1connect.CloudLinkServiceHandler.
type CloudLinkRPC struct {
	db              *sql.DB
	io              *ioworkspace.IOWorkspaceService
	us              suser.UserService
	links           *scloudlink.Reader
	files           *sfile.Reader
	flows           *sflow.FlowReader
	fileStream      eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
	flowStream      eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]
	publisher       mutation.Publisher
	deletePublisher mutation.Publisher
	logger          *slog.Logger
}

var _ cloud_linkv1connect.CloudLinkServiceHandler = (*CloudLinkRPC)(nil)

// New creates the handler. It panics on missing dependencies, like the other handlers.
func New(deps CloudLinkRPCDeps) *CloudLinkRPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("CloudLinkRPC deps validation failed: %v", err))
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &CloudLinkRPC{
		db:              deps.DB,
		io:              ioworkspace.New(deps.Queries, logger),
		us:              deps.User,
		links:           deps.Readers.CloudLink,
		files:           deps.Readers.File,
		flows:           deps.Readers.Flow,
		fileStream:      deps.Streamers.File,
		flowStream:      deps.Streamers.Flow,
		publisher:       deps.Publisher,
		deletePublisher: deps.DeletePublisher,
		logger:          logger,
	}
}

// CreateService registers the handler.
func CreateService(srv *CloudLinkRPC, options []connect.HandlerOption) (*api.Service, error) {
	p, handler := cloud_linkv1connect.NewCloudLinkServiceHandler(srv, options...)
	return &api.Service{Path: p, Handler: handler}, nil
}

// CloudLinkList returns the workspace's links, with each flow file's flows and dirty state.
func (h *CloudLinkRPC) CloudLinkList(ctx context.Context, req *connect.Request[cloudlinkv1.CloudLinkListRequest]) (*connect.Response[cloudlinkv1.CloudLinkListResponse], error) {
	workspaceID, err := parseID(req.Msg.GetWorkspaceId(), "workspace id")
	if err != nil {
		return nil, err
	}
	if err := h.checkWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}

	links, err := h.links.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*cloudlinkv1.CloudLink, 0, len(links))
	repoDirty := map[mcloudlink.Target]bool{}
	for _, l := range links {
		item, err := h.toAPI(ctx, l)
		if err != nil {
			return nil, err
		}
		if item.Dirty {
			repoDirty[l.Target().RepoTarget()] = true
		}
		items = append(items, item)
	}
	// A repo is dirty when one of its flow files is.
	for i, l := range links {
		if l.Kind == mcloudlink.KindRepo && repoDirty[l.Target()] {
			items[i].Dirty = true
		}
	}
	return connect.NewResponse(&cloudlinkv1.CloudLinkListResponse{Items: items}), nil
}

// CloudLinkUpsert imports a flow file under its repo folder, replacing what an earlier Upsert of
// the same path imported.
func (h *CloudLinkRPC) CloudLinkUpsert(ctx context.Context, req *connect.Request[cloudlinkv1.CloudLinkUpsertRequest]) (*connect.Response[cloudlinkv1.CloudLinkUpsertResponse], error) {
	msg := req.Msg
	workspaceID, err := parseID(msg.GetWorkspaceId(), "workspace id")
	if err != nil {
		return nil, err
	}
	repoTarget, err := repoTargetOf(workspaceID, msg.GetApiUrl(), msg.GetCloudWorkspaceSlug(), msg.GetRepoSlug())
	if err != nil {
		return nil, err
	}
	filePath, err := cleanFlowFilePath(msg.GetPath())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(msg.GetYamlText()) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("yaml text is required"))
	}
	if err := h.checkWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}

	// FETCH
	flowTarget := repoTarget
	flowTarget.Path = filePath
	repoLink, repoExists, err := h.getByTarget(ctx, repoTarget)
	if err != nil {
		return nil, err
	}
	flowLink, flowExists, err := h.getByTarget(ctx, flowTarget)
	if err != nil {
		return nil, err
	}
	var oldContent []mfile.File
	if flowExists {
		if !msg.GetForce() {
			dirty, err := h.isDirty(ctx, flowLink)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if dirty {
				return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New(ErrDirty))
			}
		}
		oldContent, err = h.files.ListFilesByParent(ctx, workspaceID, &flowLink.FileID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// CHECK: build everything in memory
	var created []mfile.File
	if !repoExists {
		repoLink = mcloudlink.CloudLink{
			FileID: idwrap.NewNow(), WorkspaceID: workspaceID, Kind: mcloudlink.KindRepo,
			APIURL: repoTarget.APIURL, CloudWorkspaceSlug: repoTarget.CloudWorkspaceSlug, RepoSlug: repoTarget.RepoSlug,
		}
		created = append(created, folderFile(repoLink.FileID, workspaceID, nil, repoFolderName(msg.GetRepoName(), repoTarget.RepoSlug)))
	}
	if !flowExists {
		flowLink = mcloudlink.CloudLink{
			FileID: idwrap.NewNow(), WorkspaceID: workspaceID, Kind: mcloudlink.KindFlowFile,
			APIURL: repoTarget.APIURL, CloudWorkspaceSlug: repoTarget.CloudWorkspaceSlug, RepoSlug: repoTarget.RepoSlug,
			Path: filePath,
		}
		created = append(created, folderFile(flowLink.FileID, workspaceID, &repoLink.FileID, path.Base(filePath)))
	}
	flowLink.BaseSHA = msg.GetSha()
	flowLink.SourceText = msg.GetYamlText()
	flowLink.Dirty = false
	flowLink.CleanYAML = ""

	bundle, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(msg.GetYamlText()), yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID:   workspaceID,
		GenerateFiles: true,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow file: %w", err))
	}
	// Run variables come from the clone's .env, not from environments in the file.
	bundle.Environments, bundle.EnvironmentVars = nil, nil
	// Everything the file creates goes straight into the flow file folder (as Import with
	// ParentFolderID stores it): requests point at their folder, which must exist before them.
	for i := range bundle.Files {
		bundle.Files[i].ParentID = &flowLink.FileID
	}
	for i := range bundle.HTTPRequests {
		bundle.HTTPRequests[i].FolderID = &flowLink.FileID
	}

	// ACT
	if err := h.replaceContent(ctx, replacement{
		created: created, newRepo: !repoExists, newFlow: !flowExists,
		repoLink: repoLink, flowLink: flowLink, oldContent: oldContent, bundle: bundle,
	}); err != nil {
		return nil, err
	}

	h.publishImport(workspaceID, created, bundle)

	// The clean export is read after the import committed; until then the link is clean.
	if err := h.storeBaseline(ctx, &flowLink, msg.GetYamlText()); err != nil {
		h.logger.WarnContext(ctx, "cloud link: storing the clean export failed", "file_id", flowLink.FileID.String(), "error", err)
	}

	item, err := h.toAPI(ctx, flowLink)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&cloudlinkv1.CloudLinkUpsertResponse{Link: item}), nil
}

// CloudLinkExport returns a flow file's text with the local edits applied.
func (h *CloudLinkRPC) CloudLinkExport(ctx context.Context, req *connect.Request[cloudlinkv1.CloudLinkExportRequest]) (*connect.Response[cloudlinkv1.CloudLinkExportResponse], error) {
	link, err := h.flowFileLink(ctx, req.Msg.GetFileId())
	if err != nil {
		return nil, err
	}
	text, _, err := h.flowFileText(ctx, link)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&cloudlinkv1.CloudLinkExportResponse{
		Path:     link.Path,
		BaseSha:  link.BaseSHA,
		YamlText: string(text),
	}), nil
}

// CloudLinkMarkSaved records that the exported text was written to the clone or proposed, so the
// flow file is clean again.
func (h *CloudLinkRPC) CloudLinkMarkSaved(ctx context.Context, req *connect.Request[cloudlinkv1.CloudLinkMarkSavedRequest]) (*connect.Response[emptypb.Empty], error) {
	link, err := h.flowFileLink(ctx, req.Msg.GetFileId())
	if err != nil {
		return nil, err
	}

	// FETCH: what Export returns now becomes the saved text.
	text, current, err := h.flowFileText(ctx, link)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// ACT
	baseSHA := link.BaseSHA
	if req.Msg.BaseSha != nil {
		baseSHA = req.Msg.GetBaseSha()
	}
	if err := h.writeContent(ctx, link.FileID, scloudlink.Content{
		BaseSHA: baseSHA, Dirty: false, SourceText: string(text), CleanYAML: string(current),
	}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// CloudLinkSetLocalDir links (or, with an empty dir, unlinks) a repo's local clone, creating the
// repo link when there is none yet.
func (h *CloudLinkRPC) CloudLinkSetLocalDir(ctx context.Context, req *connect.Request[cloudlinkv1.CloudLinkSetLocalDirRequest]) (*connect.Response[cloudlinkv1.CloudLinkSetLocalDirResponse], error) {
	msg := req.Msg
	workspaceID, err := parseID(msg.GetWorkspaceId(), "workspace id")
	if err != nil {
		return nil, err
	}
	repoTarget, err := repoTargetOf(workspaceID, msg.GetApiUrl(), msg.GetCloudWorkspaceSlug(), msg.GetRepoSlug())
	if err != nil {
		return nil, err
	}
	if err := h.checkWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}

	// FETCH
	link, exists, err := h.getByTarget(ctx, repoTarget)
	if err != nil {
		return nil, err
	}

	// CHECK
	var created []mfile.File
	if !exists {
		link = mcloudlink.CloudLink{
			FileID: idwrap.NewNow(), WorkspaceID: workspaceID, Kind: mcloudlink.KindRepo,
			APIURL: repoTarget.APIURL, CloudWorkspaceSlug: repoTarget.CloudWorkspaceSlug, RepoSlug: repoTarget.RepoSlug,
		}
		created = append(created, folderFile(link.FileID, workspaceID, nil, repoFolderName(msg.GetRepoName(), repoTarget.RepoSlug)))
	}
	link.LocalDir = strings.TrimSpace(msg.GetLocalDir())

	// ACT
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)
	linkWriter := scloudlink.NewWriter(tx)
	if !exists {
		if err := h.createLinkRows(ctx, tx, created, link); err != nil {
			return nil, err
		}
	} else if err := linkWriter.UpdateLocalDir(ctx, link.FileID, link.LocalDir); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	h.publishImport(workspaceID, created, nil)

	item, err := h.toAPI(ctx, link)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&cloudlinkv1.CloudLinkSetLocalDirResponse{Link: item}), nil
}

// CloudLinkUnlink removes a link and the rows it imported. Unlinking a repo removes its flow
// files too.
func (h *CloudLinkRPC) CloudLinkUnlink(ctx context.Context, req *connect.Request[cloudlinkv1.CloudLinkUnlinkRequest]) (*connect.Response[emptypb.Empty], error) {
	fileID, err := parseID(req.Msg.GetFileId(), "file id")
	if err != nil {
		return nil, err
	}

	// FETCH
	link, err := h.accessibleLink(ctx, fileID)
	if err != nil {
		return nil, err
	}
	links := []mcloudlink.CloudLink{link}
	if link.Kind == mcloudlink.KindRepo {
		if links, err = h.links.ListByRepo(ctx, link.Target()); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	var folders []mfile.File
	for _, l := range links {
		f, err := h.files.GetFile(ctx, l.FileID)
		if err != nil {
			if errors.Is(err, sfile.ErrFileNotFound) || errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		// A flow file folder sits in its repo folder, which removes it already.
		if link.Kind == mcloudlink.KindRepo && l.Kind == mcloudlink.KindFlowFile {
			continue
		}
		folders = append(folders, *f)
	}

	// ACT
	mut := mutation.New(h.db, mutation.WithPublisher(h.deletePublisher))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()
	linkWriter := scloudlink.NewWriter(mut.TX())
	for _, l := range links {
		if err := linkWriter.Delete(ctx, l.FileID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	if err := mut.DeleteFileBatch(ctx, deleteItems(folders)); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// replacement is what an Upsert writes in one transaction.
type replacement struct {
	created          []mfile.File // new repo and flow file folders
	newRepo, newFlow bool
	repoLink         mcloudlink.CloudLink
	flowLink         mcloudlink.CloudLink
	oldContent       []mfile.File // the flow file folder's rows from the previous import
	bundle           *ioworkspace.WorkspaceBundle
}

func (h *CloudLinkRPC) replaceContent(ctx context.Context, r replacement) error {
	mut := mutation.New(h.db, mutation.WithPublisher(h.deletePublisher))
	if err := mut.Begin(ctx); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()
	tx := mut.TX()

	var newLinks []mcloudlink.CloudLink
	if r.newRepo {
		newLinks = append(newLinks, r.repoLink)
	}
	if r.newFlow {
		newLinks = append(newLinks, r.flowLink)
	}
	for i, l := range newLinks {
		if err := h.createLinkRows(ctx, tx, r.created[i:i+1], l); err != nil {
			return err
		}
	}
	if !r.newFlow {
		if err := mut.DeleteFileBatch(ctx, deleteItems(r.oldContent)); err != nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("remove previous import: %w", err))
		}
		if err := scloudlink.NewWriter(tx).UpdateContent(ctx, r.flowLink.FileID, contentOf(r.flowLink)); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	opts := ioworkspace.GetDefaultImportOptions(r.flowLink.WorkspaceID)
	opts.PreserveIDs = true
	opts.ImportEnvironments = false
	opts.ParentFolderID = &r.flowLink.FileID
	if _, err := h.io.Import(ctx, tx, r.bundle, opts); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("import flow file: %w", err))
	}

	if err := mut.Commit(ctx); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	return nil
}

// createLinkRows writes a link's folder and link row.
func (h *CloudLinkRPC) createLinkRows(ctx context.Context, tx *sql.Tx, folders []mfile.File, link mcloudlink.CloudLink) error {
	fileWriter := sfile.NewWriter(tx, h.logger)
	for i := range folders {
		if err := fileWriter.CreateFile(ctx, &folders[i]); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}
	if err := scloudlink.NewWriter(tx).Create(ctx, link); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	return nil
}

// storeBaseline records the export right after an import as the flow file's clean state.
func (h *CloudLinkRPC) storeBaseline(ctx context.Context, link *mcloudlink.CloudLink, source string) error {
	clean, err := h.exportFlowFile(ctx, *link)
	if err != nil {
		return err
	}
	link.CleanYAML = string(clean)
	link.SourceText = source
	return h.writeContent(ctx, link.FileID, contentOf(*link))
}

func (h *CloudLinkRPC) writeContent(ctx context.Context, fileID idwrap.IDWrap, c scloudlink.Content) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)
	if err := scloudlink.NewWriter(tx).UpdateContent(ctx, fileID, c); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if err := tx.Commit(); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	return nil
}

// flowFileLink returns the accessible flow file link for a file id.
func (h *CloudLinkRPC) flowFileLink(ctx context.Context, rawID []byte) (mcloudlink.CloudLink, error) {
	fileID, err := parseID(rawID, "file id")
	if err != nil {
		return mcloudlink.CloudLink{}, err
	}
	link, err := h.accessibleLink(ctx, fileID)
	if err != nil {
		return mcloudlink.CloudLink{}, err
	}
	if link.Kind != mcloudlink.KindFlowFile {
		return mcloudlink.CloudLink{}, connect.NewError(connect.CodeInvalidArgument, errors.New("not a flow file link"))
	}
	return link, nil
}

// accessibleLink returns a link the caller's workspace owns. Links in other workspaces are not
// found, so their ids do not leak.
func (h *CloudLinkRPC) accessibleLink(ctx context.Context, fileID idwrap.IDWrap) (mcloudlink.CloudLink, error) {
	link, err := h.links.Get(ctx, fileID)
	if err != nil {
		if errors.Is(err, scloudlink.ErrNotFound) {
			return mcloudlink.CloudLink{}, connect.NewError(connect.CodeNotFound, errLinkNotFound)
		}
		return mcloudlink.CloudLink{}, connect.NewError(connect.CodeInternal, err)
	}
	if err := h.checkWorkspace(ctx, link.WorkspaceID); err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return mcloudlink.CloudLink{}, connect.NewError(connect.CodeNotFound, errLinkNotFound)
		}
		return mcloudlink.CloudLink{}, err
	}
	return link, nil
}

func (h *CloudLinkRPC) getByTarget(ctx context.Context, t mcloudlink.Target) (mcloudlink.CloudLink, bool, error) {
	link, err := h.links.GetByTarget(ctx, t)
	if errors.Is(err, scloudlink.ErrNotFound) {
		return mcloudlink.CloudLink{}, false, nil
	}
	if err != nil {
		return mcloudlink.CloudLink{}, false, connect.NewError(connect.CodeInternal, err)
	}
	return link, true, nil
}

func (h *CloudLinkRPC) checkWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	ok, err := h.us.CheckUserBelongsToWorkspace(ctx, userID, workspaceID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return connect.NewError(connect.CodeNotFound, errWorkspaceNotFound)
	}
	return nil
}

// toAPI converts a link, with its flows and dirty state for a flow file.
func (h *CloudLinkRPC) toAPI(ctx context.Context, l mcloudlink.CloudLink) (*cloudlinkv1.CloudLink, error) {
	item := &cloudlinkv1.CloudLink{
		FileId:             l.FileID.Bytes(),
		Kind:               cloudlinkv1.CloudLinkKind_CLOUD_LINK_KIND_REPO,
		ApiUrl:             l.APIURL,
		CloudWorkspaceSlug: l.CloudWorkspaceSlug,
		RepoSlug:           l.RepoSlug,
		Path:               l.Path,
		BaseSha:            l.BaseSHA,
		LocalDir:           l.LocalDir,
		Dirty:              l.Dirty,
		Flows:              []*cloudlinkv1.CloudLinkFlow{},
	}
	if l.Kind != mcloudlink.KindFlowFile {
		return item, nil
	}
	item.Kind = cloudlinkv1.CloudLinkKind_CLOUD_LINK_KIND_FLOW_FILE

	flowIDs, err := h.flowIDsUnder(ctx, l)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, id := range flowIDs {
		f, err := h.flows.GetFlow(ctx, id)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		item.Flows = append(item.Flows, &cloudlinkv1.CloudLinkFlow{FlowId: id.Bytes(), Name: f.Name})
	}

	dirty, err := h.isDirty(ctx, l)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	item.Dirty = dirty
	return item, nil
}

func contentOf(l mcloudlink.CloudLink) scloudlink.Content {
	return scloudlink.Content{BaseSHA: l.BaseSHA, Dirty: l.Dirty, SourceText: l.SourceText, CleanYAML: l.CleanYAML}
}

func folderFile(id, workspaceID idwrap.IDWrap, parentID *idwrap.IDWrap, name string) mfile.File {
	return mfile.File{ID: id, WorkspaceID: workspaceID, ParentID: parentID, ContentType: mfile.ContentTypeFolder, Name: name}
}

func deleteItems(files []mfile.File) []mutation.FileDeleteItem {
	items := make([]mutation.FileDeleteItem, 0, len(files))
	for _, f := range files {
		items = append(items, mutation.FileDeleteItem{ID: f.ID, WorkspaceID: f.WorkspaceID, ContentID: f.ContentID, ContentKind: f.ContentType})
	}
	return items
}

func repoFolderName(name, slug string) string {
	if n := strings.TrimSpace(name); n != "" {
		return n
	}
	return slug
}

func parseID(raw []byte, what string) (idwrap.IDWrap, error) {
	id, err := idwrap.NewFromBytes(raw)
	if err != nil {
		return idwrap.IDWrap{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid %s: %w", what, err))
	}
	return id, nil
}

func repoTargetOf(workspaceID idwrap.IDWrap, apiURL, cloudWorkspaceSlug, repoSlug string) (mcloudlink.Target, error) {
	t := mcloudlink.Target{
		WorkspaceID:        workspaceID,
		APIURL:             strings.TrimRight(strings.TrimSpace(apiURL), "/"),
		CloudWorkspaceSlug: strings.TrimSpace(cloudWorkspaceSlug),
		RepoSlug:           strings.TrimSpace(repoSlug),
	}
	if t.APIURL == "" || t.CloudWorkspaceSlug == "" || t.RepoSlug == "" {
		return t, connect.NewError(connect.CodeInvalidArgument, errors.New("api url, cloud workspace slug and repo slug are required"))
	}
	return t, nil
}

// cleanFlowFilePath accepts a repo-relative path without "..".
func cleanFlowFilePath(p string) (string, error) {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	clean := path.Clean(p)
	if p == "" || path.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("path must be a repo-relative flow file path"))
	}
	return clean, nil
}
