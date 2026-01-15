package rcredential

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	credentialv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/credential/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/credential/v1/credentialv1connect"
)

type CredentialRPC struct {
	DB *sql.DB

	cs scredential.CredentialService
	us suser.UserService
	ws sworkspace.WorkspaceService
	fs *sfile.FileService

	credReader *scredential.CredentialReader

	credStream eventstream.SyncStreamer[CredentialTopic, CredentialEvent]
}

type CredentialTopic struct {
	WorkspaceID idwrap.IDWrap
}

type CredentialEvent struct {
	Type       string
	Credential *credentialv1.Credential
}

type CredentialRPCDeps struct {
	DB        *sql.DB
	Service   scredential.CredentialService
	User      suser.UserService
	Workspace sworkspace.WorkspaceService
	File      *sfile.FileService
	Reader    *scredential.CredentialReader
	Streamer  eventstream.SyncStreamer[CredentialTopic, CredentialEvent]
}

func New(deps CredentialRPCDeps) CredentialRPC {
	return CredentialRPC{
		DB:         deps.DB,
		cs:         deps.Service,
		us:         deps.User,
		ws:         deps.Workspace,
		fs:         deps.File,
		credReader: deps.Reader,
		credStream: deps.Streamer,
	}
}

func CreateService(srv CredentialRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := credentialv1connect.NewCredentialServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (s *CredentialRPC) CredentialCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[credentialv1.CredentialCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var items []*credentialv1.Credential
	for _, ws := range workspaces {
		creds, err := s.credReader.ListCredentials(ctx, ws.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, c := range creds {
			items = append(items, converter.ToAPICredential(c))
		}
	}

	return connect.NewResponse(&credentialv1.CredentialCollectionResponse{Items: items}), nil
}

func (s *CredentialRPC) GetCredentialOpenAi(
	ctx context.Context,
	req *connect.Request[credentialv1.GetCredentialOpenAiRequest],
) (*connect.Response[credentialv1.GetCredentialOpenAiResponse], error) {
	credID, err := idwrap.NewFromBytes(req.Msg.CredentialId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	cred, err := s.credReader.GetCredential(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, cred.WorkspaceID)
	if err != nil || !belongs {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("credential not found"))
	}

	openai, err := s.credReader.GetCredentialOpenAI(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiOpenAI := converter.ToAPICredentialOpenAI(*openai)
	return connect.NewResponse(&credentialv1.GetCredentialOpenAiResponse{
		CredentialId: apiOpenAI.CredentialId,
		Token:        apiOpenAI.Token,
		BaseUrl:      apiOpenAI.BaseUrl,
	}), nil
}

func (s *CredentialRPC) GetCredentialGemini(
	ctx context.Context,
	req *connect.Request[credentialv1.GetCredentialGeminiRequest],
) (*connect.Response[credentialv1.GetCredentialGeminiResponse], error) {
	credID, err := idwrap.NewFromBytes(req.Msg.CredentialId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	cred, err := s.credReader.GetCredential(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, cred.WorkspaceID)
	if err != nil || !belongs {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("credential not found"))
	}

	gemini, err := s.credReader.GetCredentialGemini(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiGemini := converter.ToAPICredentialGemini(*gemini)
	return connect.NewResponse(&credentialv1.GetCredentialGeminiResponse{
		CredentialId: apiGemini.CredentialId,
		ApiKey:       apiGemini.ApiKey,
		BaseUrl:      apiGemini.BaseUrl,
	}), nil
}

func (s *CredentialRPC) GetCredentialAnthropic(
	ctx context.Context,
	req *connect.Request[credentialv1.GetCredentialAnthropicRequest],
) (*connect.Response[credentialv1.GetCredentialAnthropicResponse], error) {
	credID, err := idwrap.NewFromBytes(req.Msg.CredentialId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	cred, err := s.credReader.GetCredential(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, cred.WorkspaceID)
	if err != nil || !belongs {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("credential not found"))
	}

	anthropic, err := s.credReader.GetCredentialAnthropic(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiAnthropic := converter.ToAPICredentialAnthropic(*anthropic)
	return connect.NewResponse(&credentialv1.GetCredentialAnthropicResponse{
		CredentialId: apiAnthropic.CredentialId,
		ApiKey:       apiAnthropic.ApiKey,
		BaseUrl:      apiAnthropic.BaseUrl,
	}), nil
}

func (s *CredentialRPC) CredentialInsert(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// FETCH phase: Validate all items before transaction
	type credItem struct {
		credID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		cred        *mcredential.Credential
		file        *mfile.File
	}
	var items []credItem

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := idwrap.NewFromBytes(item.GetWorkspaceId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// CHECK phase: Verify workspace access
		belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, workspaceID)
		if err != nil || !belongs {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("access denied"))
		}

		// Build credential model
		cred := &mcredential.Credential{
			ID:          credID,
			WorkspaceID: workspaceID,
			Name:        item.GetName(),
			Kind:        mcredential.CredentialKind(int8(item.GetKind())), //nolint:gosec // G115: Kind is a small enum (0-2)
		}

		// Build file model to link credential into file tree
		// FileID is new, ContentID points to the credential
		fileID := idwrap.NewNow()
		file := &mfile.File{
			ID:          fileID,
			WorkspaceID: workspaceID,
			ParentID:    nil, // Root level by default
			ContentID:   &credID,
			ContentType: mfile.ContentTypeCredential,
			Name:        cred.Name,
			Order:       0, // Default order
		}

		items = append(items, credItem{
			credID:      credID,
			workspaceID: workspaceID,
			cred:        cred,
			file:        file,
		})
	}

	// ACT phase: Create File and Credential records in transaction
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Use transaction-scoped services
	csTx := s.cs.TX(tx)
	fsTx := s.fs.TX(tx)

	for _, item := range items {
		// Create credential first (content)
		if err := csTx.CreateCredential(ctx, item.cred); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Create file record to link into tree
		if err := fsTx.CreateFile(ctx, item.file); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events for real-time sync
	for _, item := range items {
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: item.workspaceID}, CredentialEvent{
				Type:       "insert",
				Credential: converter.ToAPICredential(*item.cred),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialUpdate(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// FETCH phase: Gather and validate all items before transaction
	type updateItem struct {
		credID      idwrap.IDWrap
		existing    *mcredential.Credential
		nameChanged bool
		newName     string
	}
	var items []updateItem

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing credential to check ownership
		existing, err := s.credReader.GetCredential(ctx, credID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// CHECK phase: Verify ownership
		belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, existing.WorkspaceID)
		if err != nil || !belongs {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("credential not found"))
		}

		ui := updateItem{
			credID:   credID,
			existing: existing,
		}

		// Track name change for file sync
		if item.Name != nil && *item.Name != existing.Name {
			ui.nameChanged = true
			ui.newName = *item.Name
			existing.Name = *item.Name
		}

		items = append(items, ui)
	}

	// ACT phase: Update credential and file in transaction
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)
	fsTx := s.fs.TX(tx)

	for _, item := range items {
		// Update credential
		if err := csTx.UpdateCredential(ctx, item.existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// If name changed, update the file record too
		if item.nameChanged {
			file, err := fsTx.GetFileByContentID(ctx, item.credID)
			if err == nil && file != nil {
				file.Name = item.newName
				if err := fsTx.UpdateFile(ctx, file); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
			// If file not found, that's OK - credential can exist without file entry
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events after commit
	for _, item := range items {
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: item.existing.WorkspaceID}, CredentialEvent{
				Type:       "update",
				Credential: converter.ToAPICredential(*item.existing),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialDelete(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// FETCH phase: Gather and validate all items before transaction
	type deleteItem struct {
		credID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
	}
	var items []deleteItem

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing credential
		existing, err := s.credReader.GetCredential(ctx, credID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue // Already deleted
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// CHECK phase: Verify ownership
		belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, existing.WorkspaceID)
		if err != nil || !belongs {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("credential not found"))
		}

		items = append(items, deleteItem{
			credID:      credID,
			workspaceID: existing.WorkspaceID,
		})
	}

	// ACT phase: Delete file and credential in transaction
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)
	fsTx := s.fs.TX(tx)

	for _, item := range items {
		// Delete file record first (no FK constraint from files to credentials)
		file, err := fsTx.GetFileByContentID(ctx, item.credID)
		if err == nil && file != nil {
			if err := fsTx.DeleteFile(ctx, file.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		// If file not found, that's OK - proceed with credential deletion

		// Delete credential (provider-specific cascade handled by DB FK)
		if err := csTx.DeleteCredential(ctx, item.credID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events after commit
	for _, item := range items {
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: item.workspaceID}, CredentialEvent{
				Type:       "delete",
				Credential: &credentialv1.Credential{CredentialId: item.credID.Bytes()},
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialSync(
	ctx context.Context,
	req *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[credentialv1.CredentialSyncResponse],
) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Build initial collection
	var items []*credentialv1.Credential
	for _, ws := range workspaces {
		creds, err := s.credReader.ListCredentials(ctx, ws.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		for _, c := range creds {
			items = append(items, converter.ToAPICredential(c))
		}
	}

	// Convert to sync items format
	syncItems := make([]*credentialv1.CredentialSync, 0, len(items))
	for _, item := range items {
		syncItems = append(syncItems, &credentialv1.CredentialSync{
			Value: &credentialv1.CredentialSync_ValueUnion{
				Kind: credentialv1.CredentialSync_ValueUnion_KIND_UPSERT,
				Upsert: &credentialv1.CredentialSyncUpsert{
					CredentialId: item.CredentialId,
					Name:         item.Name,
					Kind:         item.Kind,
				},
			},
		})
	}

	// Send initial collection as upsert items
	if err := stream.Send(&credentialv1.CredentialSyncResponse{
		Items: syncItems,
	}); err != nil {
		return err
	}

	// Wait for context cancellation (real-time streaming not yet implemented)
	<-ctx.Done()
	return nil
}
