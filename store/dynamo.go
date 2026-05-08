package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
)

// DDBClient is the subset of the dynamodb.Client API we use, narrowed to
// keep DynamoStore unit-testable with a fake.
type DDBClient interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

// DynamoStore persists a Snapshot as a single DynamoDB item.
//
// Schema:
//
//	Partition key: "id" (string), the only key. Always equal to "snapshot".
//	Attribute:    "payload" (string), the canonical JSON marshaling of Snapshot.
//
// Single-item design: simple, atomic writes, identical canonical content
// regardless of backend.
type DynamoStore struct {
	Client DDBClient // see DDBClient interface
	Table  string    // e.g. "ausfallplan-state"
}

// NewDynamoStore returns a DynamoStore backed by client using the given table name.
func NewDynamoStore(client DDBClient, table string) *DynamoStore {
	return &DynamoStore{Client: client, Table: table}
}

// Load fetches the snapshot item from DynamoDB.
// If the item does not exist, it returns Snapshot{}, nil (first-run case).
func (d *DynamoStore) Load(ctx context.Context) (ausfallplan.Snapshot, error) {
	out, err := d.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &d.Table,
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: "snapshot"},
		},
	})
	if err != nil {
		return ausfallplan.Snapshot{}, fmt.Errorf("dynamostore load: GetItem: %w", err)
	}

	if out.Item == nil {
		return ausfallplan.Snapshot{}, nil
	}

	payloadAttr, ok := out.Item["payload"].(*types.AttributeValueMemberS)
	if !ok {
		return ausfallplan.Snapshot{}, fmt.Errorf("dynamostore load: payload attribute missing or wrong type")
	}

	var snap ausfallplan.Snapshot
	if err := json.Unmarshal([]byte(payloadAttr.Value), &snap); err != nil {
		return ausfallplan.Snapshot{}, fmt.Errorf("dynamostore load: unmarshal payload: %w", err)
	}

	return snap, nil
}

// Save canonicalizes the snapshot and writes it as a single DynamoDB item.
// The payload attribute holds the indented JSON (same style as FileStore).
func (d *DynamoStore) Save(ctx context.Context, snap ausfallplan.Snapshot) error {
	canonical := canonicalize(snap)

	data, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return fmt.Errorf("dynamostore save: marshal: %w", err)
	}
	data = append(data, '\n')

	_, err = d.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &d.Table,
		Item: map[string]types.AttributeValue{
			"id":      &types.AttributeValueMemberS{Value: "snapshot"},
			"payload": &types.AttributeValueMemberS{Value: string(data)},
		},
	})
	if err != nil {
		return fmt.Errorf("dynamostore save: PutItem: %w", err)
	}

	return nil
}
