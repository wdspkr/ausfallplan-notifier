package store_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
	"github.com/wdspkr/ausfallplan-notifier/store"
)

func TestDynamoStore_Integration(t *testing.T) {
	endpoint := os.Getenv("DDB_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("DDB_TEST_ENDPOINT not set; skipping DynamoDB integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build a real *dynamodb.Client pointing at endpoint with static dummy creds.
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("local", "local", ""),
		),
	)
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}

	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = &endpoint
	})

	// Unique table name to avoid collisions between parallel test runs.
	tableName := fmt.Sprintf("ausfallplan-integration-%d-%d",
		time.Now().UnixNano(), rand.Intn(10000))

	// Create the table.
	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: &tableName,
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: strPtr("id"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: strPtr("id"),
				KeyType:       types.KeyTypeHash,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: &tableName,
		})
	})

	// Wait for the table to become ACTIVE (poll up to 10 s).
	waitCtx, waitCancel := context.WithTimeout(ctx, 10*time.Second)
	defer waitCancel()
	for {
		desc, err := client.DescribeTable(waitCtx, &dynamodb.DescribeTableInput{
			TableName: &tableName,
		})
		if err != nil {
			t.Fatalf("describe table: %v", err)
		}
		if desc.Table.TableStatus == types.TableStatusActive {
			break
		}
		select {
		case <-waitCtx.Done():
			t.Fatal("timed out waiting for table to become ACTIVE")
		case <-time.After(200 * time.Millisecond):
		}
	}

	ds := store.NewDynamoStore(client, tableName)

	// Load from empty table → empty snapshot.
	snap, err := ds.Load(ctx)
	if err != nil {
		t.Fatalf("Load (empty): %v", err)
	}
	if len(snap.Entries) != 0 || len(snap.Infos) != 0 {
		t.Fatalf("expected empty snapshot, got %+v", snap)
	}

	// Save a sample snapshot (deliberately non-canonical order).
	sample := ausfallplan.Snapshot{
		Entries: []ausfallplan.Entry{
			{Day: time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), Hour: "1. Stunde", Class: "6b", Information: "Vertretung"},
			{Day: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC), Hour: "3. Stunde", Class: "3d", Information: "Ausfall"},
		},
		Infos: []ausfallplan.Info{
			{Text: "Schulausflug 4c"},
			{Text: "Morgen kein Sport"},
		},
	}
	if err := ds.Save(ctx, sample); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Load back and verify it matches the canonical form.
	loaded, err := ds.Load(ctx)
	if err != nil {
		t.Fatalf("Load (after save): %v", err)
	}

	if len(loaded.Entries) != len(sample.Entries) {
		t.Fatalf("expected %d entries, got %d", len(sample.Entries), len(loaded.Entries))
	}

	// Canonical order: day1 (2026-05-08) entry first.
	expectedFirst := sample.Entries[1] // 3d entry (day1)
	if !loaded.Entries[0].Day.Equal(expectedFirst.Day) ||
		loaded.Entries[0].Class != expectedFirst.Class {
		t.Errorf("first entry after round-trip: got %+v, want %+v", loaded.Entries[0], expectedFirst)
	}

	if len(loaded.Infos) != len(sample.Infos) {
		t.Fatalf("expected %d infos, got %d", len(sample.Infos), len(loaded.Infos))
	}
	// Canonical order: "Morgen kein Sport" < "Schulausflug 4c"
	if loaded.Infos[0].Text != "Morgen kein Sport" {
		t.Errorf("first info after round-trip: got %q, want %q", loaded.Infos[0].Text, "Morgen kein Sport")
	}
}

func strPtr(s string) *string { return &s }
