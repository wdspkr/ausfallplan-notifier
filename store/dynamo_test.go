package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
	"github.com/wdspkr/ausfallplan-notifier/store"
)

// fakeDDB is a minimal in-memory fake DDBClient for unit tests.
type fakeDDB struct {
	// getOutput is returned by GetItem.
	getOutput *dynamodb.GetItemOutput
	// getErr is returned by GetItem when non-nil.
	getErr error
	// putErr is returned by PutItem when non-nil.
	putErr error

	// capturedPut holds the last PutItemInput passed to PutItem.
	capturedPut *dynamodb.PutItemInput

	// currentItem is updated by PutItem and read by GetItem in RoundTrip tests.
	currentItem map[string]types.AttributeValue
}

func (f *fakeDDB) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	// If currentItem is set (round-trip mode), use it.
	if f.currentItem != nil {
		return &dynamodb.GetItemOutput{Item: f.currentItem}, nil
	}
	if f.getOutput != nil {
		return f.getOutput, nil
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (f *fakeDDB) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	if f.putErr != nil {
		return nil, f.putErr
	}
	f.capturedPut = in
	// Mirror put item into currentItem for round-trip support.
	f.currentItem = in.Item
	return &dynamodb.PutItemOutput{}, nil
}

// ── test fixtures ─────────────────────────────────────────────────────────────

var (
	day1DT = time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	day2DT = time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
)

// nonCanonicalSnap has entries in reverse-canonical order so Save must re-sort them.
var nonCanonicalSnap = ausfallplan.Snapshot{
	Entries: []ausfallplan.Entry{
		{Day: day2DT, Hour: "1. Stunde", Class: "6b", Information: "Vertretung"},
		{Day: day1DT, Hour: "3. Stunde", Class: "3d", Information: "Ausfall"},
	},
	Infos: []ausfallplan.Info{
		{Text: "Schulausflug 4c"},
		{Text: "Morgen kein Sport"},
	},
}

// canonicalSnap is the same data but in canonical (sorted) order.
var canonicalSnap = ausfallplan.Snapshot{
	Entries: []ausfallplan.Entry{
		{Day: day1DT, Hour: "3. Stunde", Class: "3d", Information: "Ausfall"},
		{Day: day2DT, Hour: "1. Stunde", Class: "6b", Information: "Vertretung"},
	},
	Infos: []ausfallplan.Info{
		{Text: "Morgen kein Sport"},
		{Text: "Schulausflug 4c"},
	},
}

const tableName = "ausfallplan-state"

// ── unit tests ────────────────────────────────────────────────────────────────

func TestDynamoStore_Load_NoItem(t *testing.T) {
	fake := &fakeDDB{getOutput: &dynamodb.GetItemOutput{Item: nil}}
	ds := store.NewDynamoStore(fake, tableName)

	snap, err := ds.Load(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(snap.Entries) != 0 {
		t.Errorf("expected empty Entries, got %d", len(snap.Entries))
	}
	if len(snap.Infos) != 0 {
		t.Errorf("expected empty Infos, got %d", len(snap.Infos))
	}
}

func TestDynamoStore_Load_ParsesPayload(t *testing.T) {
	// Encode the canonical snapshot as the JSON that would be stored.
	payload, err := json.MarshalIndent(canonicalSnap, "", "  ")
	if err != nil {
		t.Fatalf("marshal canonical snap: %v", err)
	}
	payload = append(payload, '\n')

	fake := &fakeDDB{
		getOutput: &dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"id":      &types.AttributeValueMemberS{Value: "snapshot"},
				"payload": &types.AttributeValueMemberS{Value: string(payload)},
			},
		},
	}
	ds := store.NewDynamoStore(fake, tableName)

	got, err := ds.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(got.Entries) != len(canonicalSnap.Entries) {
		t.Fatalf("expected %d entries, got %d", len(canonicalSnap.Entries), len(got.Entries))
	}
	for i, e := range got.Entries {
		exp := canonicalSnap.Entries[i]
		if !e.Day.Equal(exp.Day) || e.Hour != exp.Hour || e.Class != exp.Class || e.Information != exp.Information {
			t.Errorf("entry[%d] mismatch: got %+v, want %+v", i, e, exp)
		}
	}
	if len(got.Infos) != len(canonicalSnap.Infos) {
		t.Fatalf("expected %d infos, got %d", len(canonicalSnap.Infos), len(got.Infos))
	}
	for i, info := range got.Infos {
		if info.Text != canonicalSnap.Infos[i].Text {
			t.Errorf("info[%d] mismatch: got %q, want %q", i, info.Text, canonicalSnap.Infos[i].Text)
		}
	}
}

func TestDynamoStore_Load_GetItemError(t *testing.T) {
	fake := &fakeDDB{getErr: errors.New("connection refused")}
	ds := store.NewDynamoStore(fake, tableName)

	_, err := ds.Load(context.Background())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestDynamoStore_Save_PutsCanonicalJSON(t *testing.T) {
	fake := &fakeDDB{}
	ds := store.NewDynamoStore(fake, tableName)

	if err := ds.Save(context.Background(), nonCanonicalSnap); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if fake.capturedPut == nil {
		t.Fatal("PutItem was not called")
	}
	input := fake.capturedPut

	// TableName must match.
	if input.TableName == nil || *input.TableName != tableName {
		t.Errorf("TableName: got %v, want %q", input.TableName, tableName)
	}

	// "id" attribute must be the string "snapshot".
	idAttr, ok := input.Item["id"].(*types.AttributeValueMemberS)
	if !ok {
		t.Fatalf("id attribute is not *AttributeValueMemberS: %T", input.Item["id"])
	}
	if idAttr.Value != "snapshot" {
		t.Errorf("id attribute value: got %q, want %q", idAttr.Value, "snapshot")
	}

	// "payload" attribute must be the canonical JSON.
	payloadAttr, ok := input.Item["payload"].(*types.AttributeValueMemberS)
	if !ok {
		t.Fatalf("payload attribute is not *AttributeValueMemberS: %T", input.Item["payload"])
	}

	var got ausfallplan.Snapshot
	if err := json.Unmarshal([]byte(payloadAttr.Value), &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	// Entries must be in canonical order (day1 before day2).
	if len(got.Entries) != len(canonicalSnap.Entries) {
		t.Fatalf("expected %d entries in payload, got %d", len(canonicalSnap.Entries), len(got.Entries))
	}
	for i, e := range got.Entries {
		exp := canonicalSnap.Entries[i]
		if !e.Day.Equal(exp.Day) || e.Hour != exp.Hour || e.Class != exp.Class || e.Information != exp.Information {
			t.Errorf("canonical entry[%d] mismatch: got %+v, want %+v", i, e, exp)
		}
	}

	// Infos must be in canonical order.
	if len(got.Infos) != len(canonicalSnap.Infos) {
		t.Fatalf("expected %d infos in payload, got %d", len(canonicalSnap.Infos), len(got.Infos))
	}
	for i, info := range got.Infos {
		if info.Text != canonicalSnap.Infos[i].Text {
			t.Errorf("canonical info[%d] mismatch: got %q, want %q", i, info.Text, canonicalSnap.Infos[i].Text)
		}
	}
}

func TestDynamoStore_Save_PutItemError(t *testing.T) {
	fake := &fakeDDB{putErr: errors.New("throughput exceeded")}
	ds := store.NewDynamoStore(fake, tableName)

	err := ds.Save(context.Background(), nonCanonicalSnap)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestDynamoStore_RoundTrip(t *testing.T) {
	fake := &fakeDDB{}
	ds := store.NewDynamoStore(fake, tableName)
	ctx := context.Background()

	if err := ds.Save(ctx, nonCanonicalSnap); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := ds.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Result must equal the canonical form of the original snapshot.
	if len(got.Entries) != len(canonicalSnap.Entries) {
		t.Fatalf("expected %d entries after round-trip, got %d", len(canonicalSnap.Entries), len(got.Entries))
	}
	for i, e := range got.Entries {
		exp := canonicalSnap.Entries[i]
		if !e.Day.Equal(exp.Day) || e.Hour != exp.Hour || e.Class != exp.Class || e.Information != exp.Information {
			t.Errorf("round-trip entry[%d] mismatch: got %+v, want %+v", i, e, exp)
		}
	}
	if len(got.Infos) != len(canonicalSnap.Infos) {
		t.Fatalf("expected %d infos after round-trip, got %d", len(canonicalSnap.Infos), len(got.Infos))
	}
	for i, info := range got.Infos {
		if info.Text != canonicalSnap.Infos[i].Text {
			t.Errorf("round-trip info[%d] mismatch: got %q, want %q", i, info.Text, canonicalSnap.Infos[i].Text)
		}
	}
}
