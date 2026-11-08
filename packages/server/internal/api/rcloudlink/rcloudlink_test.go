package rcloudlink

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rfile"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rflowv2"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scloudlink"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	cloudlinkv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/cloud_link/v1"
)

const (
	testAPIURL   = "https://app.stresseur.test"
	testCloudWS  = "acme"
	testRepoSlug = "acme-shop"
	testRepoName = "acme/shop"
)

type recordedEvents struct {
	mu     sync.Mutex
	events []mutation.Event
}

func (r *recordedEvents) PublishAll(events []mutation.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, events...)
}

func (r *recordedEvents) count(entity mutation.EntityType, op mutation.Operation) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e.Entity == entity && e.Op == op {
			n++
		}
	}
	return n
}

type fixture struct {
	t           *testing.T
	ctx         context.Context
	base        *testutil.BaseDBQueries
	services    testutil.BaseTestServices
	h           *CloudLinkRPC
	workspaceID idwrap.IDWrap
	inserts     *recordedEvents
	deletes     *recordedEvents
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	base := testutil.CreateBaseDBWithFK(ctx, t)
	t.Cleanup(base.Close)
	services := base.GetBaseServices()

	userID := idwrap.NewNow()
	providerID := "test-" + userID.String()
	require.NoError(t, services.UserService.CreateUser(ctx, &muser.User{
		ID: userID, Email: userID.String() + "@example.com", ProviderID: &providerID,
		ProviderType: muser.MagicLink, Status: muser.Active,
	}))
	workspaceID, err := services.CreateTempCollection(ctx, userID, "Studio")
	require.NoError(t, err)

	fileStream := memory.NewInMemorySyncStreamer[rfile.FileTopic, rfile.FileEvent]()
	flowStream := memory.NewInMemorySyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]()
	t.Cleanup(fileStream.Shutdown)
	t.Cleanup(flowStream.Shutdown)

	inserts, deletes := &recordedEvents{}, &recordedEvents{}
	h := New(CloudLinkRPCDeps{
		DB:      base.DB,
		Queries: base.Queries,
		User:    services.UserService,
		Readers: CloudLinkRPCReaders{
			CloudLink: scloudlink.NewReader(base.DB),
			File:      sfile.NewReader(base.DB, base.Logger()),
			Flow:      sflow.NewFlowReader(base.DB),
		},
		Streamers:       CloudLinkRPCStreamers{File: fileStream, Flow: flowStream},
		Publisher:       inserts,
		DeletePublisher: deletes,
		Logger:          base.Logger(),
	})

	return &fixture{
		t: t, ctx: mwauth.CreateAuthedContext(ctx, userID), base: base, services: services,
		h: h, workspaceID: workspaceID, inserts: inserts, deletes: deletes,
	}
}

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(data)
}

func (f *fixture) upsert(path, sha, text string, force bool) (*cloudlinkv1.CloudLink, error) {
	f.t.Helper()
	res, err := f.h.CloudLinkUpsert(f.ctx, connect.NewRequest(&cloudlinkv1.CloudLinkUpsertRequest{
		WorkspaceId: f.workspaceID.Bytes(), ApiUrl: testAPIURL, CloudWorkspaceSlug: testCloudWS,
		RepoSlug: testRepoSlug, RepoName: testRepoName, Path: path, Sha: sha, YamlText: text, Force: &force,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetLink(), nil
}

func (f *fixture) mustUpsert(path, sha, text string) *cloudlinkv1.CloudLink {
	f.t.Helper()
	link, err := f.upsert(path, sha, text, false)
	require.NoError(f.t, err)
	return link
}

func (f *fixture) list() []*cloudlinkv1.CloudLink {
	f.t.Helper()
	res, err := f.h.CloudLinkList(f.ctx, connect.NewRequest(&cloudlinkv1.CloudLinkListRequest{WorkspaceId: f.workspaceID.Bytes()}))
	require.NoError(f.t, err)
	return res.Msg.GetItems()
}

func (f *fixture) linkByPath(path string) *cloudlinkv1.CloudLink {
	f.t.Helper()
	for _, l := range f.list() {
		if l.GetPath() == path {
			return l
		}
	}
	f.t.Fatalf("no link for %q", path)
	return nil
}

func (f *fixture) export(fileID []byte) string {
	f.t.Helper()
	res, err := f.h.CloudLinkExport(f.ctx, connect.NewRequest(&cloudlinkv1.CloudLinkExportRequest{FileId: fileID}))
	require.NoError(f.t, err)
	return res.Msg.GetYamlText()
}

func (f *fixture) workspaceCounts() (flows, requests, files int) {
	f.t.Helper()
	fl, err := sflow.NewFlowReader(f.base.DB).GetFlowsByWorkspaceID(f.ctx, f.workspaceID)
	require.NoError(f.t, err)
	hs, err := f.services.HttpService.GetByWorkspaceID(f.ctx, f.workspaceID)
	require.NoError(f.t, err)
	fs, err := sfile.NewReader(f.base.DB, f.base.Logger()).ListFilesByWorkspace(f.ctx, f.workspaceID)
	require.NoError(f.t, err)
	return len(fl), len(hs), len(fs)
}

// editRequest changes the URL of the workspace's HTTP request with that name, as the editor does.
func (f *fixture) editRequest(name, suffix string) {
	f.t.Helper()
	hs, err := f.services.HttpService.GetByWorkspaceID(f.ctx, f.workspaceID)
	require.NoError(f.t, err)
	for i := range hs {
		if hs[i].Name == name {
			h := hs[i]
			h.Url += suffix
			require.NoError(f.t, f.services.HttpService.Update(f.ctx, &h))
			return
		}
	}
	f.t.Fatalf("no request %q", name)
}

func (f *fixture) requestURL(name string) string {
	f.t.Helper()
	hs, err := f.services.HttpService.GetByWorkspaceID(f.ctx, f.workspaceID)
	require.NoError(f.t, err)
	for _, h := range hs {
		if h.Name == name {
			return h.Url
		}
	}
	f.t.Fatalf("no request %q", name)
	return ""
}

func changedLines(a, b string) []string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	if len(al) != len(bl) {
		return []string{fmt.Sprintf("line count %d != %d", len(al), len(bl))}
	}
	var out []string
	for i := range al {
		if al[i] != bl[i] {
			out = append(out, bl[i])
		}
	}
	return out
}

const adminPath = "api-tests/admin-catalog-crud.yamlflow.yaml"

func TestUpsertReplacesInsteadOfDuplicating(t *testing.T) {
	f := newFixture(t)
	text := readTestdata(t, "admin-catalog-crud.yamlflow.yaml")

	first := f.mustUpsert(adminPath, "sha-1", text)
	require.Equal(t, cloudlinkv1.CloudLinkKind_CLOUD_LINK_KIND_FLOW_FILE, first.GetKind())
	require.Len(t, first.GetFlows(), 1)
	require.Equal(t, "admin-catalog-crud", first.GetFlows()[0].GetName())
	require.False(t, first.GetDirty())
	flows, requests, files := f.workspaceCounts()
	require.Equal(t, 1, flows)
	require.Equal(t, 7, requests)
	require.Positive(t, f.inserts.count(mutation.EntityFlowNode, mutation.OpInsert))

	second := f.mustUpsert(adminPath, "sha-2", text)
	require.Equal(t, first.GetFileId(), second.GetFileId(), "the flow file keeps its folder")
	require.Equal(t, "sha-2", second.GetBaseSha())
	require.NotEqual(t, first.GetFlows()[0].GetFlowId(), second.GetFlows()[0].GetFlowId())

	flows2, requests2, files2 := f.workspaceCounts()
	require.Equal(t, flows, flows2)
	require.Equal(t, requests, requests2)
	require.Equal(t, files, files2)
	require.Positive(t, f.deletes.count(mutation.EntityFile, mutation.OpDelete))

	links := f.list()
	require.Len(t, links, 2, "one repo link and one flow file link")
	var repo *cloudlinkv1.CloudLink
	for _, l := range links {
		if l.GetKind() == cloudlinkv1.CloudLinkKind_CLOUD_LINK_KIND_REPO {
			repo = l
		}
	}
	require.NotNil(t, repo)
	require.Empty(t, repo.GetPath())

	// The repo folder is a root row (the client hides it, and with it the flow file).
	repoFolder, err := sfile.NewReader(f.base.DB, f.base.Logger()).GetFile(f.ctx, idwrap.NewFromBytesMust(repo.GetFileId()))
	require.NoError(t, err)
	require.Nil(t, repoFolder.ParentID)
	require.Equal(t, testRepoName, repoFolder.Name)
}

func TestUpsertDirtyGuard(t *testing.T) {
	f := newFixture(t)
	text := readTestdata(t, "admin-catalog-crud.yamlflow.yaml")
	link := f.mustUpsert(adminPath, "sha-1", text)
	original := f.requestURL("GetTags")

	f.editRequest("GetTags", "?edited=1")
	for _, l := range f.list() {
		require.True(t, l.GetDirty(), "%s link is dirty after an edit", l.GetKind())
	}

	_, err := f.upsert(adminPath, "sha-2", text, false)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	require.Contains(t, err.Error(), ErrDirty)
	require.Equal(t, original+"?edited=1", f.requestURL("GetTags"), "a refused upsert keeps the edit")

	forced, err := f.upsert(adminPath, "sha-2", text, true)
	require.NoError(t, err)
	require.False(t, forced.GetDirty())
	require.Equal(t, link.GetFileId(), forced.GetFileId())
	require.Equal(t, original, f.requestURL("GetTags"), "force takes main")
	for _, l := range f.list() {
		require.False(t, l.GetDirty())
	}
}

func TestExportKeepsSourceAndMarkSaved(t *testing.T) {
	f := newFixture(t)
	text := readTestdata(t, "admin-catalog-crud.yamlflow.yaml")
	link := f.mustUpsert(adminPath, "sha-1", text)

	// Untouched: the fetched text, byte for byte.
	require.Equal(t, text, f.export(link.GetFileId()))

	// One edit changes one line; the header comment and load: stay.
	f.editRequest("GetTags", "?edited=1")
	edited := f.export(link.GetFileId())
	diff := changedLines(text, edited)
	require.Len(t, diff, 1, "export:\n%s", edited)
	require.Contains(t, diff[0], "/api/tags?edited=1")
	require.True(t, strings.HasPrefix(edited, "# generated by stresseur-recorder"))
	require.Contains(t, edited, "\nload:\n  - name: smoke\n")

	savedSHA := "sha-saved"
	_, err := f.h.CloudLinkMarkSaved(f.ctx, connect.NewRequest(&cloudlinkv1.CloudLinkMarkSavedRequest{
		FileId: link.GetFileId(), BaseSha: &savedSHA,
	}))
	require.NoError(t, err)
	saved := f.linkByPath(adminPath)
	require.False(t, saved.GetDirty())
	require.Equal(t, "sha-saved", saved.GetBaseSha())
	require.Equal(t, edited, f.export(link.GetFileId()))

	// The next edit is applied to the saved text.
	f.editRequest("GetCategories", "?page=2")
	next := f.export(link.GetFileId())
	diff = changedLines(edited, next)
	require.Len(t, diff, 1, "export:\n%s", next)
	require.Contains(t, diff[0], "/api/categories?page=2")
	require.True(t, f.linkByPath(adminPath).GetDirty())
}

func TestExportKeepsCleanupSteps(t *testing.T) {
	f := newFixture(t)
	text := readTestdata(t, "items-crud.yamlflow.yaml")
	link := f.mustUpsert("api-tests/items-crud.yamlflow.yaml", "sha-1", text)
	require.Equal(t, text, f.export(link.GetFileId()))

	f.editRequest("GetItem", "?full=1")
	edited := f.export(link.GetFileId())
	require.Len(t, changedLines(text, edited), 1)
	require.Contains(t, edited, "    cleanup:\n      - request:\n          name: DeleteItem\n")
}

func TestSetLocalDirAndUnlink(t *testing.T) {
	f := newFixture(t)
	setDir := func(dir string) *cloudlinkv1.CloudLink {
		res, err := f.h.CloudLinkSetLocalDir(f.ctx, connect.NewRequest(&cloudlinkv1.CloudLinkSetLocalDirRequest{
			WorkspaceId: f.workspaceID.Bytes(), ApiUrl: testAPIURL, CloudWorkspaceSlug: testCloudWS,
			RepoSlug: testRepoSlug, RepoName: testRepoName, LocalDir: dir,
		}))
		require.NoError(t, err)
		return res.Msg.GetLink()
	}

	// No repo link yet: SetLocalDir creates it.
	repo := setDir("/home/me/shop")
	require.Equal(t, cloudlinkv1.CloudLinkKind_CLOUD_LINK_KIND_REPO, repo.GetKind())
	require.Equal(t, "/home/me/shop", repo.GetLocalDir())
	again := setDir("/home/me/shop-2")
	require.Equal(t, repo.GetFileId(), again.GetFileId())
	require.Len(t, f.list(), 1)

	// Flow files go in the same repo folder.
	admin := f.mustUpsert(adminPath, "sha-1", readTestdata(t, "admin-catalog-crud.yamlflow.yaml"))
	items := f.mustUpsert("api-tests/items-crud.yamlflow.yaml", "sha-1", readTestdata(t, "items-crud.yamlflow.yaml"))
	require.Len(t, f.list(), 3)
	require.Equal(t, "/home/me/shop-2", f.linkByPath("").GetLocalDir())
	folder, err := sfile.NewReader(f.base.DB, f.base.Logger()).GetFile(f.ctx, idwrap.NewFromBytesMust(admin.GetFileId()))
	require.NoError(t, err)
	require.Equal(t, repo.GetFileId(), folder.ParentID.Bytes())

	// Unlinking a flow file removes only it.
	_, err = f.h.CloudLinkUnlink(f.ctx, connect.NewRequest(&cloudlinkv1.CloudLinkUnlinkRequest{FileId: items.GetFileId()}))
	require.NoError(t, err)
	require.Len(t, f.list(), 2)
	flows, _, _ := f.workspaceCounts()
	require.Equal(t, 1, flows)

	// Unlinking the repo removes everything under it.
	_, err = f.h.CloudLinkUnlink(f.ctx, connect.NewRequest(&cloudlinkv1.CloudLinkUnlinkRequest{FileId: repo.GetFileId()}))
	require.NoError(t, err)
	require.Empty(t, f.list())
	flows, requests, files := f.workspaceCounts()
	require.Zero(t, flows)
	require.Zero(t, requests)
	require.Zero(t, files)

	_, err = f.h.CloudLinkExport(f.ctx, connect.NewRequest(&cloudlinkv1.CloudLinkExportRequest{FileId: admin.GetFileId()}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestCloudLinkAccessAndValidation(t *testing.T) {
	f := newFixture(t)
	link := f.mustUpsert(adminPath, "sha-1", readTestdata(t, "admin-catalog-crud.yamlflow.yaml"))

	other := mwauth.CreateAuthedContext(context.Background(), idwrap.NewNow())
	_, err := f.h.CloudLinkList(other, connect.NewRequest(&cloudlinkv1.CloudLinkListRequest{WorkspaceId: f.workspaceID.Bytes()}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	_, err = f.h.CloudLinkExport(other, connect.NewRequest(&cloudlinkv1.CloudLinkExportRequest{FileId: link.GetFileId()}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	for _, p := range []string{"", "../x.yamlflow.yaml", "/etc/x.yamlflow.yaml"} {
		_, err = f.upsert(p, "sha", "flows: []", false)
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "path %q", p)
	}
	_, err = f.upsert("api-tests/bad.yamlflow.yaml", "sha", "flows: [", false)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	// A repo link cannot be exported.
	_, err = f.h.CloudLinkExport(f.ctx, connect.NewRequest(&cloudlinkv1.CloudLinkExportRequest{FileId: f.linkByPath("").GetFileId()}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
