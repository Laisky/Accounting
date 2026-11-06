package audit

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/storage"
)

// TestSQLRepositoryAuditRoundTripSQLite exercises the relational audit repository end to end
// against an on-disk migrated SQLite database.
func TestSQLRepositoryAuditRoundTripSQLite(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, "sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))

	runAuditSQLRepositoryRoundTrip(t, db)
}

// TestSQLRepositoryAuditRoundTripPostgres runs the same round-trip against postgres when
// DATABASE_URL is configured, mirroring the storage integration test gating.
func TestSQLRepositoryAuditRoundTripPostgres(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(databaseURL) == "" {
		t.Skip("DATABASE_URL not set; skipping postgres audit repository integration test")
	}

	ctx := context.Background()
	db, err := storage.Open(ctx, "postgres", databaseURL, "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))

	runAuditSQLRepositoryRoundTrip(t, db)
}

func runAuditSQLRepositoryRoundTrip(t *testing.T, db *storage.DB) {
	t.Helper()
	ctx := context.Background()

	// Start from a clean chain so the seq and hash-chain assertions are deterministic even on
	// the shared postgres test database.
	_, err := db.SQLDB().ExecContext(ctx, `DELETE FROM audit_events`)
	require.NoError(t, err)

	repo, err := NewSQLRepository(db)
	require.NoError(t, err)

	// Tail on an empty chain reports ErrNotFound, matching the legacy stores.
	_, err = repo.Tail(ctx)
	require.ErrorIs(t, err, ErrNotFound)

	// Deterministic microsecond-safe clock so timestamps round-trip exactly on both dialects.
	var tick int64
	base := time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC)
	svc := NewService(repo).WithClock(func() time.Time {
		n := atomic.AddInt64(&tick, 1)
		return base.Add(time.Duration(n) * time.Second)
	})

	actorA := "actor-a-" + uuid.NewString()
	actorB := "actor-b-" + uuid.NewString()
	requests := []RecordRequest{
		{ActorID: actorA, ActorEmail: "A@Example.com", Action: ActionAuthLogin, TargetType: "session", TargetID: "s1", Metadata: map[string]string{"ip": "10.0.0.1"}},
		{ActorID: actorB, Action: ActionAuthLogin, TargetType: "session", TargetID: "s2"},
		{ActorID: actorA, Action: ActionBookCreated, TargetType: "book", TargetID: "b1", Metadata: map[string]string{"name": "Household"}},
		{ActorID: actorA, Action: ActionAuthLogout, TargetType: "session", TargetID: "s1"},
		{ActorID: actorB, Action: ActionEntryCreated, TargetType: "entry", TargetID: "e1"},
	}

	recorded := make([]Event, 0, len(requests))
	prevHash := ""
	for index, request := range requests {
		event, err := svc.Record(ctx, request)
		require.NoError(t, err)
		require.Equal(t, int64(index+1), event.Seq, "seq is strictly increasing from 1")
		require.Equal(t, prevHash, event.PrevHash, "prev hash links to the previous event")
		require.NotEmpty(t, event.Hash)
		prevHash = event.Hash
		recorded = append(recorded, event)
	}

	// Tail returns the latest event with its reconstructed hash intact.
	tail, err := repo.Tail(ctx)
	require.NoError(t, err)
	require.Equal(t, recorded[len(recorded)-1].ID, tail.ID)
	require.Equal(t, int64(len(requests)), tail.Seq)
	require.Equal(t, recorded[len(recorded)-1].Hash, tail.Hash)
	require.Equal(t, recorded[len(recorded)-1].PrevHash, tail.PrevHash)

	// AllEvents returns everything newest first and forms a valid tamper-evident chain.
	all, err := repo.AllEvents(ctx)
	require.NoError(t, err)
	require.Len(t, all, len(requests))
	for index := 0; index < len(all)-1; index++ {
		require.Greater(t, all[index].Seq, all[index+1].Seq, "events are ordered newest first")
	}
	require.NoError(t, VerifyChain(all))

	// Round-trip fidelity: reconstructed hashes equal the record-time hashes.
	require.Equal(t, recorded[4].ID, all[0].ID)
	require.Equal(t, recorded[4].Hash, all[0].Hash)
	require.Equal(t, recorded[2].Hash, all[2].Hash)
	require.Equal(t, map[string]string{"name": "Household"}, all[2].Metadata, "non-empty metadata round-trips")
	require.Nil(t, all[1].Metadata, "empty metadata round-trips to nil")

	// EventsByActor returns only that actor's events, newest first, with correct chain hashes.
	actorAEvents, err := repo.EventsByActor(ctx, actorA)
	require.NoError(t, err)
	require.Equal(t, []string{recorded[3].ID, recorded[2].ID, recorded[0].ID},
		[]string{actorAEvents[0].ID, actorAEvents[1].ID, actorAEvents[2].ID})
	for _, event := range actorAEvents {
		require.Equal(t, actorA, event.ActorID)
	}
	require.Equal(t, "a@example.com", actorAEvents[2].ActorEmail, "actor email is lowercased and preserved")
	require.Equal(t, recorded[0].Hash, actorAEvents[2].Hash)

	// Service pagination over the actor-scoped list.
	page1, err := svc.List(ctx, ListRequest{ActorID: actorA, Page: 1, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, 3, page1.Total)
	require.Equal(t, []string{recorded[3].ID, recorded[2].ID},
		[]string{page1.Items[0].ID, page1.Items[1].ID})
	page2, err := svc.List(ctx, ListRequest{ActorID: actorA, Page: 2, PageSize: 2})
	require.NoError(t, err)
	require.Len(t, page2.Items, 1)
	require.Equal(t, recorded[0].ID, page2.Items[0].ID)

	// ListAll pagination returns every event newest first and stays chain-valid.
	listAll, err := svc.ListAll(ctx, ListRequest{Page: 1, PageSize: 100})
	require.NoError(t, err)
	require.Equal(t, len(requests), listAll.Total)
	require.Len(t, listAll.Items, len(requests))
	require.Equal(t, recorded[4].ID, listAll.Items[0].ID)
	require.NoError(t, VerifyChain(listAll.Items))
}
