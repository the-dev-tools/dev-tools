package rcredential

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	credentialv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/credential/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/credential/v1/credentialv1connect"
)

// Event type constants
const (
	eventTypeInsert = "insert"
	eventTypeUpdate = "update"
	eventTypeDelete = "delete"
)

type CredentialRPC struct {
	DB *sql.DB

	cs scredential.CredentialService
	us suser.UserService
	ws sworkspace.WorkspaceService

	credReader *scredential.CredentialReader

	credStream      eventstream.SyncStreamer[CredentialTopic, CredentialEvent]
	openAiStream    eventstream.SyncStreamer[CredentialOpenAiTopic, CredentialOpenAiEvent]
	geminiStream    eventstream.SyncStreamer[CredentialGeminiTopic, CredentialGeminiEvent]
	anthropicStream eventstream.SyncStreamer[CredentialAnthropicTopic, CredentialAnthropicEvent]

	publisher mutation.Publisher // Unified publisher for cascade delete events
}

// --- Credential Topics and Events ---

type CredentialTopic struct {
	WorkspaceID idwrap.IDWrap
}

type CredentialEvent struct {
	Type       string
	Credential *credentialv1.Credential
}

type CredentialOpenAiTopic struct {
	WorkspaceID idwrap.IDWrap
}

type CredentialOpenAiEvent struct {
	Type   string
	Secret *credentialv1.CredentialOpenAi
}

type CredentialGeminiTopic struct {
	WorkspaceID idwrap.IDWrap
}

type CredentialGeminiEvent struct {
	Type   string
	Secret *credentialv1.CredentialGemini
}

type CredentialAnthropicTopic struct {
	WorkspaceID idwrap.IDWrap
}

type CredentialAnthropicEvent struct {
	Type   string
	Secret *credentialv1.CredentialAnthropic
}

// --- Dependencies ---

type CredentialRPCServices struct {
	Credential scredential.CredentialService
	User       suser.UserService
	Workspace  sworkspace.WorkspaceService
}

func (s *CredentialRPCServices) Validate() error {
	return nil
}

type CredentialRPCReaders struct {
	Credential *scredential.CredentialReader
}

func (r *CredentialRPCReaders) Validate() error {
	if r.Credential == nil {
		return fmt.Errorf("credential reader is required")
	}
	return nil
}

type CredentialRPCStreamers struct {
	Credential eventstream.SyncStreamer[CredentialTopic, CredentialEvent]
	OpenAi     eventstream.SyncStreamer[CredentialOpenAiTopic, CredentialOpenAiEvent]
	Gemini     eventstream.SyncStreamer[CredentialGeminiTopic, CredentialGeminiEvent]
	Anthropic  eventstream.SyncStreamer[CredentialAnthropicTopic, CredentialAnthropicEvent]
}

func (s *CredentialRPCStreamers) Validate() error {
	if s.Credential == nil {
		return fmt.Errorf("credential stream is required")
	}
	if s.OpenAi == nil {
		return fmt.Errorf("openai stream is required")
	}
	if s.Gemini == nil {
		return fmt.Errorf("gemini stream is required")
	}
	if s.Anthropic == nil {
		return fmt.Errorf("anthropic stream is required")
	}
	return nil
}

type CredentialRPCDeps struct {
	DB        *sql.DB
	Services  CredentialRPCServices
	Readers   CredentialRPCReaders
	Streamers CredentialRPCStreamers
	Publisher mutation.Publisher // Unified publisher for cascade delete events
}

func (d *CredentialRPCDeps) Validate() error {
	if d.DB == nil {
		return fmt.Errorf("db is required")
	}
	if err := d.Services.Validate(); err != nil {
		return err
	}
	if err := d.Readers.Validate(); err != nil {
		return err
	}
	if err := d.Streamers.Validate(); err != nil {
		return err
	}
	return nil
}

func New(deps CredentialRPCDeps) CredentialRPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("CredentialRPC Deps validation failed: %v", err))
	}

	return CredentialRPC{
		DB:              deps.DB,
		cs:              deps.Services.Credential,
		us:              deps.Services.User,
		ws:              deps.Services.Workspace,
		credReader:      deps.Readers.Credential,
		credStream:      deps.Streamers.Credential,
		openAiStream:    deps.Streamers.OpenAi,
		geminiStream:    deps.Streamers.Gemini,
		anthropicStream: deps.Streamers.Anthropic,
		publisher:       deps.Publisher,
	}
}

func CreateService(srv CredentialRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := credentialv1connect.NewCredentialServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// --- Helper Methods ---

// getAccessibleWorkspaces returns all workspaces the user has access to
func (s *CredentialRPC) getAccessibleWorkspaces(ctx context.Context) ([]mworkspace.Workspace, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return workspaces, nil
}

// getCredentialWorkspaceID retrieves the workspace ID for a credential and verifies access
func (s *CredentialRPC) getCredentialWorkspaceID(ctx context.Context, credID idwrap.IDWrap) (idwrap.IDWrap, error) {
	cred, err := s.credReader.GetCredential(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, connect.NewError(connect.CodeNotFound, err)
		}
		return idwrap.IDWrap{}, connect.NewError(connect.CodeInternal, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return idwrap.IDWrap{}, connect.NewError(connect.CodeUnauthenticated, err)
	}

	belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, cred.WorkspaceID)
	if err != nil || !belongs {
		return idwrap.IDWrap{}, connect.NewError(connect.CodeNotFound, errors.New("credential not found"))
	}

	return cred.WorkspaceID, nil
}

// --- Credential Collection CRUD+Sync ---

func (s *CredentialRPC) CredentialCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[credentialv1.CredentialCollectionResponse], error) {
	workspaces, err := s.getAccessibleWorkspaces(ctx)
	if err != nil {
		return nil, err
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
			Kind:        converter.ToModelCredentialKind(item.GetKind()),
		}

		items = append(items, credItem{
			credID:      credID,
			workspaceID: workspaceID,
			cred:        cred,
		})
	}

	// ACT phase: Create Credential records in transaction
	// Note: File creation is handled by frontend via File collection
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	// Track provider-specific records created for sync events
	type providerRecord struct {
		kind        mcredential.CredentialKind
		workspaceID idwrap.IDWrap
		openai      *mcredential.CredentialOpenAI
		gemini      *mcredential.CredentialGemini
		anthropic   *mcredential.CredentialAnthropic
	}
	var providerRecords []providerRecord

	for _, item := range items {
		if err := csTx.CreateCredential(ctx, item.cred); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Auto-create provider-specific record with empty defaults
		// This ensures the record exists when the frontend tries to update it
		pr := providerRecord{kind: item.cred.Kind, workspaceID: item.workspaceID}
		switch item.cred.Kind {
		case mcredential.CREDENTIAL_KIND_OPENAI:
			pr.openai = &mcredential.CredentialOpenAI{
				CredentialID: item.credID,
				Token:        "",
			}
			if err := csTx.CreateCredentialOpenAI(ctx, pr.openai); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		case mcredential.CREDENTIAL_KIND_GEMINI:
			pr.gemini = &mcredential.CredentialGemini{
				CredentialID: item.credID,
				ApiKey:       "",
			}
			if err := csTx.CreateCredentialGemini(ctx, pr.gemini); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		case mcredential.CREDENTIAL_KIND_ANTHROPIC:
			pr.anthropic = &mcredential.CredentialAnthropic{
				CredentialID: item.credID,
				ApiKey:       "",
			}
			if err := csTx.CreateCredentialAnthropic(ctx, pr.anthropic); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		providerRecords = append(providerRecords, pr)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events for real-time sync
	for _, item := range items {
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: item.workspaceID}, CredentialEvent{
				Type:       eventTypeInsert,
				Credential: converter.ToAPICredential(*item.cred),
			})
		}
	}

	// Publish provider-specific sync events
	for _, pr := range providerRecords {
		switch pr.kind {
		case mcredential.CREDENTIAL_KIND_OPENAI:
			if s.openAiStream != nil && pr.openai != nil {
				s.openAiStream.Publish(CredentialOpenAiTopic{WorkspaceID: pr.workspaceID}, CredentialOpenAiEvent{
					Type:   eventTypeInsert,
					Secret: converter.ToAPICredentialOpenAI(*pr.openai),
				})
			}
		case mcredential.CREDENTIAL_KIND_GEMINI:
			if s.geminiStream != nil && pr.gemini != nil {
				s.geminiStream.Publish(CredentialGeminiTopic{WorkspaceID: pr.workspaceID}, CredentialGeminiEvent{
					Type:   eventTypeInsert,
					Secret: converter.ToAPICredentialGemini(*pr.gemini),
				})
			}
		case mcredential.CREDENTIAL_KIND_ANTHROPIC:
			if s.anthropicStream != nil && pr.anthropic != nil {
				s.anthropicStream.Publish(CredentialAnthropicTopic{WorkspaceID: pr.workspaceID}, CredentialAnthropicEvent{
					Type:   eventTypeInsert,
					Secret: converter.ToAPICredentialAnthropic(*pr.anthropic),
				})
			}
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
		credID   idwrap.IDWrap
		existing *mcredential.Credential
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

		// Apply updates
		if item.Name != nil {
			existing.Name = *item.Name
		}

		items = append(items, updateItem{
			credID:   credID,
			existing: existing,
		})
	}

	// ACT phase: Update credential in transaction
	// Note: File updates are handled by frontend via File collection
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	for _, item := range items {
		if err := csTx.UpdateCredential(ctx, item.existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events after commit
	for _, item := range items {
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: item.existing.WorkspaceID}, CredentialEvent{
				Type:       eventTypeUpdate,
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

	// ACT phase: Delete credential in transaction
	// Note: File deletion is handled by frontend via File collection
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	for _, item := range items {
		if err := csTx.DeleteCredential(ctx, item.credID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish credential events after commit
	for _, item := range items {
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: item.workspaceID}, CredentialEvent{
				Type:       eventTypeDelete,
				Credential: &credentialv1.Credential{CredentialId: item.credID.Bytes()},
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[credentialv1.CredentialSyncResponse],
) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	return s.streamCredentialSync(ctx, userID, stream.Send)
}

func (s *CredentialRPC) streamCredentialSync(
	ctx context.Context,
	userID idwrap.IDWrap,
	send func(*credentialv1.CredentialSyncResponse) error,
) error {
	// Build set of accessible workspace IDs for filtering
	var workspaceSet sync.Map

	filter := func(topic CredentialTopic) bool {
		_, ok := workspaceSet.Load(topic.WorkspaceID.String())
		return ok
	}

	// Load initial workspaces
	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	for _, ws := range workspaces {
		workspaceSet.Store(ws.ID.String(), true)
	}

	// Real-time streaming: subscribe to credential events
	if s.credStream == nil {
		<-ctx.Done()
		return nil
	}

	eventCh, err := s.credStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Stream events as they come
	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-eventCh:
			if !ok {
				return nil
			}

			var syncItem *credentialv1.CredentialSync
			switch evt.Payload.Type {
			case eventTypeInsert, eventTypeUpdate:
				syncItem = &credentialv1.CredentialSync{
					Value: &credentialv1.CredentialSync_ValueUnion{
						Kind: credentialv1.CredentialSync_ValueUnion_KIND_UPSERT,
						Upsert: &credentialv1.CredentialSyncUpsert{
							CredentialId: evt.Payload.Credential.CredentialId,
							Name:         evt.Payload.Credential.Name,
							Kind:         evt.Payload.Credential.Kind,
						},
					},
				}
			case eventTypeDelete:
				syncItem = &credentialv1.CredentialSync{
					Value: &credentialv1.CredentialSync_ValueUnion{
						Kind:   credentialv1.CredentialSync_ValueUnion_KIND_DELETE,
						Delete: &credentialv1.CredentialSyncDelete{CredentialId: evt.Payload.Credential.CredentialId},
					},
				}
			}

			if syncItem != nil {
				if err := send(&credentialv1.CredentialSyncResponse{
					Items: []*credentialv1.CredentialSync{syncItem},
				}); err != nil {
					return err
				}
			}
		}
	}
}

// --- CredentialOpenAi Collection CRUD+Sync ---

func (s *CredentialRPC) CredentialOpenAiCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[credentialv1.CredentialOpenAiCollectionResponse], error) {
	workspaces, err := s.getAccessibleWorkspaces(ctx)
	if err != nil {
		return nil, err
	}

	var items []*credentialv1.CredentialOpenAi
	for _, ws := range workspaces {
		creds, err := s.credReader.ListCredentials(ctx, ws.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, c := range creds {
			if c.Kind != mcredential.CREDENTIAL_KIND_OPENAI {
				continue
			}
			openai, err := s.credReader.GetCredentialOpenAI(ctx, c.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, converter.ToAPICredentialOpenAI(*openai))
		}
	}

	return connect.NewResponse(&credentialv1.CredentialOpenAiCollectionResponse{Items: items}), nil
}

func (s *CredentialRPC) CredentialOpenAiInsert(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialOpenAiInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		credID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		openai      *mcredential.CredentialOpenAI
		exists      bool
	}
	var validatedItems []insertData

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.getCredentialWorkspaceID(ctx, credID)
		if err != nil {
			return nil, err
		}

		// Check if record already exists (auto-created by CredentialInsert) - BEFORE transaction
		_, existErr := s.credReader.GetCredentialOpenAI(ctx, credID)
		var exists bool
		switch {
		case existErr == nil:
			exists = true
		case errors.Is(existErr, sql.ErrNoRows):
			exists = false
		default:
			return nil, connect.NewError(connect.CodeInternal, existErr)
		}

		openai := &mcredential.CredentialOpenAI{
			CredentialID: credID,
			Token:        item.GetToken(),
		}
		// Only set BaseUrl if it's non-empty (empty string means use provider default)
		if item.BaseUrl != nil && *item.BaseUrl != "" {
			openai.BaseUrl = item.BaseUrl
		}

		validatedItems = append(validatedItems, insertData{
			credID:      credID,
			workspaceID: workspaceID,
			openai:      openai,
			exists:      exists,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	for _, data := range validatedItems {
		if data.exists {
			// Record exists, update it instead
			if err := csTx.UpdateCredentialOpenAI(ctx, data.openai); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		} else {
			// Record doesn't exist, create it
			if err := csTx.CreateCredentialOpenAI(ctx, data.openai); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events after commit
	for _, data := range validatedItems {
		if s.openAiStream != nil {
			s.openAiStream.Publish(CredentialOpenAiTopic{WorkspaceID: data.workspaceID}, CredentialOpenAiEvent{
				Type:   eventTypeInsert,
				Secret: converter.ToAPICredentialOpenAI(*data.openai),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialOpenAiUpdate(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialOpenAiUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		credID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		updated     *mcredential.CredentialOpenAI
	}
	var validatedItems []updateData

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.getCredentialWorkspaceID(ctx, credID)
		if err != nil {
			return nil, err
		}

		existing, err := s.credReader.GetCredentialOpenAI(ctx, credID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updated := *existing

		if item.Token != nil {
			updated.Token = *item.Token
		}

		if item.BaseUrl != nil {
			switch item.BaseUrl.Kind {
			case credentialv1.CredentialOpenAiUpdate_BaseUrlUnion_KIND_VALUE:
				// Empty string means use provider default (same as unset)
				if item.BaseUrl.Value != nil && *item.BaseUrl.Value != "" {
					updated.BaseUrl = item.BaseUrl.Value
				} else {
					updated.BaseUrl = nil
				}
			case credentialv1.CredentialOpenAiUpdate_BaseUrlUnion_KIND_UNSET:
				updated.BaseUrl = nil
			}
		}

		validatedItems = append(validatedItems, updateData{
			credID:      credID,
			workspaceID: workspaceID,
			updated:     &updated,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	for _, data := range validatedItems {
		if err := csTx.UpdateCredentialOpenAI(ctx, data.updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events after commit
	for _, data := range validatedItems {
		if s.openAiStream != nil {
			s.openAiStream.Publish(CredentialOpenAiTopic{WorkspaceID: data.workspaceID}, CredentialOpenAiEvent{
				Type:   eventTypeUpdate,
				Secret: converter.ToAPICredentialOpenAI(*data.updated),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialOpenAiDelete(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialOpenAiDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	type deleteData struct {
		credID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
	}
	var validatedItems []deleteData

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.getCredentialWorkspaceID(ctx, credID)
		if err != nil {
			// If credential doesn't exist, skip (already deleted)
			if connect.CodeOf(err) == connect.CodeNotFound {
				continue
			}
			return nil, err
		}

		validatedItems = append(validatedItems, deleteData{
			credID:      credID,
			workspaceID: workspaceID,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// Note: We delete the parent credential which cascades to delete the OpenAI secret
	// Note: File deletion is handled by frontend via File collection
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	for _, data := range validatedItems {
		// Delete credential (cascades to OpenAI secret)
		if err := csTx.DeleteCredential(ctx, data.credID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events after commit
	for _, data := range validatedItems {
		if s.openAiStream != nil {
			s.openAiStream.Publish(CredentialOpenAiTopic{WorkspaceID: data.workspaceID}, CredentialOpenAiEvent{
				Type:   eventTypeDelete,
				Secret: &credentialv1.CredentialOpenAi{CredentialId: data.credID.Bytes()},
			})
		}
		// Also publish credential delete event
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: data.workspaceID}, CredentialEvent{
				Type:       eventTypeDelete,
				Credential: &credentialv1.Credential{CredentialId: data.credID.Bytes()},
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialOpenAiSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[credentialv1.CredentialOpenAiSyncResponse],
) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	return s.streamCredentialOpenAiSync(ctx, userID, stream.Send)
}

func (s *CredentialRPC) streamCredentialOpenAiSync(
	ctx context.Context,
	userID idwrap.IDWrap,
	send func(*credentialv1.CredentialOpenAiSyncResponse) error,
) error {
	var workspaceSet sync.Map

	filter := func(topic CredentialOpenAiTopic) bool {
		_, ok := workspaceSet.Load(topic.WorkspaceID.String())
		return ok
	}

	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	for _, ws := range workspaces {
		workspaceSet.Store(ws.ID.String(), true)
	}

	if s.openAiStream == nil {
		<-ctx.Done()
		return nil
	}

	eventCh, err := s.openAiStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-eventCh:
			if !ok {
				return nil
			}

			var syncItem *credentialv1.CredentialOpenAiSync
			switch evt.Payload.Type {
			case eventTypeInsert, eventTypeUpdate:
				syncItem = &credentialv1.CredentialOpenAiSync{
					Value: &credentialv1.CredentialOpenAiSync_ValueUnion{
						Kind: credentialv1.CredentialOpenAiSync_ValueUnion_KIND_UPSERT,
						Upsert: &credentialv1.CredentialOpenAiSyncUpsert{
							CredentialId: evt.Payload.Secret.CredentialId,
							Token:        evt.Payload.Secret.Token,
							BaseUrl:      evt.Payload.Secret.BaseUrl,
						},
					},
				}
			case eventTypeDelete:
				syncItem = &credentialv1.CredentialOpenAiSync{
					Value: &credentialv1.CredentialOpenAiSync_ValueUnion{
						Kind:   credentialv1.CredentialOpenAiSync_ValueUnion_KIND_DELETE,
						Delete: &credentialv1.CredentialOpenAiSyncDelete{CredentialId: evt.Payload.Secret.CredentialId},
					},
				}
			}

			if syncItem != nil {
				if err := send(&credentialv1.CredentialOpenAiSyncResponse{
					Items: []*credentialv1.CredentialOpenAiSync{syncItem},
				}); err != nil {
					return err
				}
			}
		}
	}
}

// --- CredentialGemini Collection CRUD+Sync ---

func (s *CredentialRPC) CredentialGeminiCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[credentialv1.CredentialGeminiCollectionResponse], error) {
	workspaces, err := s.getAccessibleWorkspaces(ctx)
	if err != nil {
		return nil, err
	}

	var items []*credentialv1.CredentialGemini
	for _, ws := range workspaces {
		creds, err := s.credReader.ListCredentials(ctx, ws.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, c := range creds {
			if c.Kind != mcredential.CREDENTIAL_KIND_GEMINI {
				continue
			}
			gemini, err := s.credReader.GetCredentialGemini(ctx, c.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, converter.ToAPICredentialGemini(*gemini))
		}
	}

	return connect.NewResponse(&credentialv1.CredentialGeminiCollectionResponse{Items: items}), nil
}

func (s *CredentialRPC) CredentialGeminiInsert(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialGeminiInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		credID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		gemini      *mcredential.CredentialGemini
		exists      bool
	}
	var validatedItems []insertData

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.getCredentialWorkspaceID(ctx, credID)
		if err != nil {
			return nil, err
		}

		// Check if record already exists (auto-created by CredentialInsert) - BEFORE transaction
		_, existErr := s.credReader.GetCredentialGemini(ctx, credID)
		var exists bool
		switch {
		case existErr == nil:
			exists = true
		case errors.Is(existErr, sql.ErrNoRows):
			exists = false
		default:
			return nil, connect.NewError(connect.CodeInternal, existErr)
		}

		gemini := &mcredential.CredentialGemini{
			CredentialID: credID,
			ApiKey:       item.GetApiKey(),
		}
		// Only set BaseUrl if it's non-empty (empty string means use provider default)
		if item.BaseUrl != nil && *item.BaseUrl != "" {
			gemini.BaseUrl = item.BaseUrl
		}

		validatedItems = append(validatedItems, insertData{
			credID:      credID,
			workspaceID: workspaceID,
			gemini:      gemini,
			exists:      exists,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	for _, data := range validatedItems {
		if data.exists {
			// Record exists, update it instead
			if err := csTx.UpdateCredentialGemini(ctx, data.gemini); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		} else {
			// Record doesn't exist, create it
			if err := csTx.CreateCredentialGemini(ctx, data.gemini); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, data := range validatedItems {
		if s.geminiStream != nil {
			s.geminiStream.Publish(CredentialGeminiTopic{WorkspaceID: data.workspaceID}, CredentialGeminiEvent{
				Type:   eventTypeInsert,
				Secret: converter.ToAPICredentialGemini(*data.gemini),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialGeminiUpdate(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialGeminiUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		credID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		updated     *mcredential.CredentialGemini
	}
	var validatedItems []updateData

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.getCredentialWorkspaceID(ctx, credID)
		if err != nil {
			return nil, err
		}

		existing, err := s.credReader.GetCredentialGemini(ctx, credID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updated := *existing

		if item.ApiKey != nil {
			updated.ApiKey = *item.ApiKey
		}

		if item.BaseUrl != nil {
			switch item.BaseUrl.Kind {
			case credentialv1.CredentialGeminiUpdate_BaseUrlUnion_KIND_VALUE:
				// Empty string means use provider default (same as unset)
				if item.BaseUrl.Value != nil && *item.BaseUrl.Value != "" {
					updated.BaseUrl = item.BaseUrl.Value
				} else {
					updated.BaseUrl = nil
				}
			case credentialv1.CredentialGeminiUpdate_BaseUrlUnion_KIND_UNSET:
				updated.BaseUrl = nil
			}
		}

		validatedItems = append(validatedItems, updateData{
			credID:      credID,
			workspaceID: workspaceID,
			updated:     &updated,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	for _, data := range validatedItems {
		if err := csTx.UpdateCredentialGemini(ctx, data.updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, data := range validatedItems {
		if s.geminiStream != nil {
			s.geminiStream.Publish(CredentialGeminiTopic{WorkspaceID: data.workspaceID}, CredentialGeminiEvent{
				Type:   eventTypeUpdate,
				Secret: converter.ToAPICredentialGemini(*data.updated),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialGeminiDelete(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialGeminiDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	type deleteData struct {
		credID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
	}
	var validatedItems []deleteData

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.getCredentialWorkspaceID(ctx, credID)
		if err != nil {
			if connect.CodeOf(err) == connect.CodeNotFound {
				continue
			}
			return nil, err
		}

		validatedItems = append(validatedItems, deleteData{
			credID:      credID,
			workspaceID: workspaceID,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// Note: File deletion is handled by frontend via File collection
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	for _, data := range validatedItems {
		if err := csTx.DeleteCredential(ctx, data.credID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, data := range validatedItems {
		if s.geminiStream != nil {
			s.geminiStream.Publish(CredentialGeminiTopic{WorkspaceID: data.workspaceID}, CredentialGeminiEvent{
				Type:   eventTypeDelete,
				Secret: &credentialv1.CredentialGemini{CredentialId: data.credID.Bytes()},
			})
		}
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: data.workspaceID}, CredentialEvent{
				Type:       eventTypeDelete,
				Credential: &credentialv1.Credential{CredentialId: data.credID.Bytes()},
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialGeminiSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[credentialv1.CredentialGeminiSyncResponse],
) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	return s.streamCredentialGeminiSync(ctx, userID, stream.Send)
}

func (s *CredentialRPC) streamCredentialGeminiSync(
	ctx context.Context,
	userID idwrap.IDWrap,
	send func(*credentialv1.CredentialGeminiSyncResponse) error,
) error {
	var workspaceSet sync.Map

	filter := func(topic CredentialGeminiTopic) bool {
		_, ok := workspaceSet.Load(topic.WorkspaceID.String())
		return ok
	}

	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	for _, ws := range workspaces {
		workspaceSet.Store(ws.ID.String(), true)
	}

	if s.geminiStream == nil {
		<-ctx.Done()
		return nil
	}

	eventCh, err := s.geminiStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-eventCh:
			if !ok {
				return nil
			}

			var syncItem *credentialv1.CredentialGeminiSync
			switch evt.Payload.Type {
			case eventTypeInsert, eventTypeUpdate:
				syncItem = &credentialv1.CredentialGeminiSync{
					Value: &credentialv1.CredentialGeminiSync_ValueUnion{
						Kind: credentialv1.CredentialGeminiSync_ValueUnion_KIND_UPSERT,
						Upsert: &credentialv1.CredentialGeminiSyncUpsert{
							CredentialId: evt.Payload.Secret.CredentialId,
							ApiKey:       evt.Payload.Secret.ApiKey,
							BaseUrl:      evt.Payload.Secret.BaseUrl,
						},
					},
				}
			case eventTypeDelete:
				syncItem = &credentialv1.CredentialGeminiSync{
					Value: &credentialv1.CredentialGeminiSync_ValueUnion{
						Kind:   credentialv1.CredentialGeminiSync_ValueUnion_KIND_DELETE,
						Delete: &credentialv1.CredentialGeminiSyncDelete{CredentialId: evt.Payload.Secret.CredentialId},
					},
				}
			}

			if syncItem != nil {
				if err := send(&credentialv1.CredentialGeminiSyncResponse{
					Items: []*credentialv1.CredentialGeminiSync{syncItem},
				}); err != nil {
					return err
				}
			}
		}
	}
}

// --- CredentialAnthropic Collection CRUD+Sync ---

func (s *CredentialRPC) CredentialAnthropicCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[credentialv1.CredentialAnthropicCollectionResponse], error) {
	workspaces, err := s.getAccessibleWorkspaces(ctx)
	if err != nil {
		return nil, err
	}

	var items []*credentialv1.CredentialAnthropic
	for _, ws := range workspaces {
		creds, err := s.credReader.ListCredentials(ctx, ws.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, c := range creds {
			if c.Kind != mcredential.CREDENTIAL_KIND_ANTHROPIC {
				continue
			}
			anthropic, err := s.credReader.GetCredentialAnthropic(ctx, c.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, converter.ToAPICredentialAnthropic(*anthropic))
		}
	}

	return connect.NewResponse(&credentialv1.CredentialAnthropicCollectionResponse{Items: items}), nil
}

func (s *CredentialRPC) CredentialAnthropicInsert(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialAnthropicInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		credID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		anthropic   *mcredential.CredentialAnthropic
		exists      bool
	}
	var validatedItems []insertData

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.getCredentialWorkspaceID(ctx, credID)
		if err != nil {
			return nil, err
		}

		// Check if record already exists (auto-created by CredentialInsert) - BEFORE transaction
		_, existErr := s.credReader.GetCredentialAnthropic(ctx, credID)
		var exists bool
		switch {
		case existErr == nil:
			exists = true
		case errors.Is(existErr, sql.ErrNoRows):
			exists = false
		default:
			return nil, connect.NewError(connect.CodeInternal, existErr)
		}

		anthropic := &mcredential.CredentialAnthropic{
			CredentialID: credID,
			ApiKey:       item.GetApiKey(),
		}
		// Only set BaseUrl if it's non-empty (empty string means use provider default)
		if item.BaseUrl != nil && *item.BaseUrl != "" {
			anthropic.BaseUrl = item.BaseUrl
		}

		validatedItems = append(validatedItems, insertData{
			credID:      credID,
			workspaceID: workspaceID,
			anthropic:   anthropic,
			exists:      exists,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	for _, data := range validatedItems {
		if data.exists {
			// Record exists, update it instead
			if err := csTx.UpdateCredentialAnthropic(ctx, data.anthropic); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		} else {
			// Record doesn't exist, create it
			if err := csTx.CreateCredentialAnthropic(ctx, data.anthropic); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, data := range validatedItems {
		if s.anthropicStream != nil {
			s.anthropicStream.Publish(CredentialAnthropicTopic{WorkspaceID: data.workspaceID}, CredentialAnthropicEvent{
				Type:   eventTypeInsert,
				Secret: converter.ToAPICredentialAnthropic(*data.anthropic),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialAnthropicUpdate(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialAnthropicUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		credID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		updated     *mcredential.CredentialAnthropic
	}
	var validatedItems []updateData

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.getCredentialWorkspaceID(ctx, credID)
		if err != nil {
			return nil, err
		}

		existing, err := s.credReader.GetCredentialAnthropic(ctx, credID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updated := *existing

		if item.ApiKey != nil {
			updated.ApiKey = *item.ApiKey
		}

		if item.BaseUrl != nil {
			switch item.BaseUrl.Kind {
			case credentialv1.CredentialAnthropicUpdate_BaseUrlUnion_KIND_VALUE:
				// Empty string means use provider default (same as unset)
				if item.BaseUrl.Value != nil && *item.BaseUrl.Value != "" {
					updated.BaseUrl = item.BaseUrl.Value
				} else {
					updated.BaseUrl = nil
				}
			case credentialv1.CredentialAnthropicUpdate_BaseUrlUnion_KIND_UNSET:
				updated.BaseUrl = nil
			}
		}

		validatedItems = append(validatedItems, updateData{
			credID:      credID,
			workspaceID: workspaceID,
			updated:     &updated,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	for _, data := range validatedItems {
		if err := csTx.UpdateCredentialAnthropic(ctx, data.updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, data := range validatedItems {
		if s.anthropicStream != nil {
			s.anthropicStream.Publish(CredentialAnthropicTopic{WorkspaceID: data.workspaceID}, CredentialAnthropicEvent{
				Type:   eventTypeUpdate,
				Secret: converter.ToAPICredentialAnthropic(*data.updated),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialAnthropicDelete(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialAnthropicDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	type deleteData struct {
		credID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
	}
	var validatedItems []deleteData

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.getCredentialWorkspaceID(ctx, credID)
		if err != nil {
			if connect.CodeOf(err) == connect.CodeNotFound {
				continue
			}
			return nil, err
		}

		validatedItems = append(validatedItems, deleteData{
			credID:      credID,
			workspaceID: workspaceID,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// Note: File deletion is handled by frontend via File collection
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	csTx := s.cs.TX(tx)

	for _, data := range validatedItems {
		if err := csTx.DeleteCredential(ctx, data.credID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, data := range validatedItems {
		if s.anthropicStream != nil {
			s.anthropicStream.Publish(CredentialAnthropicTopic{WorkspaceID: data.workspaceID}, CredentialAnthropicEvent{
				Type:   eventTypeDelete,
				Secret: &credentialv1.CredentialAnthropic{CredentialId: data.credID.Bytes()},
			})
		}
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: data.workspaceID}, CredentialEvent{
				Type:       eventTypeDelete,
				Credential: &credentialv1.Credential{CredentialId: data.credID.Bytes()},
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialAnthropicSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[credentialv1.CredentialAnthropicSyncResponse],
) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	return s.streamCredentialAnthropicSync(ctx, userID, stream.Send)
}

func (s *CredentialRPC) streamCredentialAnthropicSync(
	ctx context.Context,
	userID idwrap.IDWrap,
	send func(*credentialv1.CredentialAnthropicSyncResponse) error,
) error {
	var workspaceSet sync.Map

	filter := func(topic CredentialAnthropicTopic) bool {
		_, ok := workspaceSet.Load(topic.WorkspaceID.String())
		return ok
	}

	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	for _, ws := range workspaces {
		workspaceSet.Store(ws.ID.String(), true)
	}

	if s.anthropicStream == nil {
		<-ctx.Done()
		return nil
	}

	eventCh, err := s.anthropicStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-eventCh:
			if !ok {
				return nil
			}

			var syncItem *credentialv1.CredentialAnthropicSync
			switch evt.Payload.Type {
			case eventTypeInsert, eventTypeUpdate:
				syncItem = &credentialv1.CredentialAnthropicSync{
					Value: &credentialv1.CredentialAnthropicSync_ValueUnion{
						Kind: credentialv1.CredentialAnthropicSync_ValueUnion_KIND_UPSERT,
						Upsert: &credentialv1.CredentialAnthropicSyncUpsert{
							CredentialId: evt.Payload.Secret.CredentialId,
							ApiKey:       evt.Payload.Secret.ApiKey,
							BaseUrl:      evt.Payload.Secret.BaseUrl,
						},
					},
				}
			case eventTypeDelete:
				syncItem = &credentialv1.CredentialAnthropicSync{
					Value: &credentialv1.CredentialAnthropicSync_ValueUnion{
						Kind:   credentialv1.CredentialAnthropicSync_ValueUnion_KIND_DELETE,
						Delete: &credentialv1.CredentialAnthropicSyncDelete{CredentialId: evt.Payload.Secret.CredentialId},
					},
				}
			}

			if syncItem != nil {
				if err := send(&credentialv1.CredentialAnthropicSyncResponse{
					Items: []*credentialv1.CredentialAnthropicSync{syncItem},
				}); err != nil {
					return err
				}
			}
		}
	}
}
