package database

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStore struct {
	client        *mongo.Client
	database      *mongo.Database
	rulesColl     *mongo.Collection
	templatesColl *mongo.Collection
}

// NewMongoStore creates a new MongoStore with the given connection string and database name.
func NewMongoStore(ctx context.Context, connectionString, dbName string) (*MongoStore, error) {
	clientOptions := options.Client().ApplyURI(connectionString)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	db := client.Database(dbName)
	return &MongoStore{
		client:        client,
		database:      db,
		rulesColl:     db.Collection("rules"),
		templatesColl: db.Collection("templates"),
	}, nil
}

// Close closes the MongoDB connection.
func (s *MongoStore) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

// RuleStore Implementation

// CreateRule saves a new rule to MongoDB.
func (s *MongoStore) CreateRule(ctx context.Context, rule *Rule) error {
	if rule.ID == "" {
		rule.ID = primitive.NewObjectID().Hex()
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}
	rule.UpdatedAt = time.Now()

	_, err := s.rulesColl.InsertOne(ctx, rule)
	return err
}

// GetRule retrieves a rule by ID from MongoDB.
func (s *MongoStore) GetRule(ctx context.Context, id string) (*Rule, error) {
	var rule Rule
	err := s.rulesColl.FindOne(ctx, bson.M{"_id": id}).Decode(&rule)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("rule not found")
		}
		return nil, err
	}
	return &rule, nil
}

// ListRules retrieves a paginated list of rules from MongoDB.
func (s *MongoStore) ListRules(ctx context.Context, offset, limit int) ([]*Rule, error) {
	opts := options.Find().SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := s.rulesColl.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []*Rule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// UpdateRule updates an existing rule in MongoDB.
func (s *MongoStore) UpdateRule(ctx context.Context, id string, rule *Rule) error {
	rule.UpdatedAt = time.Now()
	update := bson.M{
		"$set": bson.M{
			"templateName": rule.TemplateName,
			"parameters":   rule.Parameters,
			"updatedAt":    rule.UpdatedAt,
		},
	}
	result, err := s.rulesColl.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return errors.New("rule not found")
	}
	return nil
}

// DeleteRule removes a rule from MongoDB.
func (s *MongoStore) DeleteRule(ctx context.Context, id string) error {
	result, err := s.rulesColl.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return errors.New("rule not found")
	}
	return nil
}

// TemplateProvider Implementation

type templateDoc struct {
	ID      string `bson:"_id"`
	Type    string `bson:"type"`
	Content string `bson:"content"`
}

// GetSchema retrieves a schema by name from MongoDB.
func (s *MongoStore) GetSchema(ctx context.Context, name string) (string, error) {
	var doc templateDoc
	// We assume the ID is exactly the name provided.
	// The type filter ensures we get the schema, not the template if they share IDs (though they shouldn't).
	err := s.templatesColl.FindOne(ctx, bson.M{"_id": name, "type": "schema"}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", errors.New("schema not found")
		}
		return "", err
	}
	return doc.Content, nil
}

// GetTemplate retrieves a template by name from MongoDB.
func (s *MongoStore) GetTemplate(ctx context.Context, name string) (string, error) {
	var doc templateDoc
	// We assume the ID is exactly the name provided.
	err := s.templatesColl.FindOne(ctx, bson.M{"_id": name, "type": "template"}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", errors.New("template not found")
		}
		return "", err
	}
	return doc.Content, nil
}

// CreateSchema saves a new schema to MongoDB.
func (s *MongoStore) CreateSchema(ctx context.Context, name, content string) error {
	_, err := s.templatesColl.UpdateOne(
		ctx,
		bson.M{"_id": name, "type": "schema"},
		bson.M{"$set": bson.M{"content": content}},
		options.Update().SetUpsert(true),
	)
	return err
}

// CreateTemplate saves a new template to MongoDB.
func (s *MongoStore) CreateTemplate(ctx context.Context, name, content string) error {
	_, err := s.templatesColl.UpdateOne(
		ctx,
		bson.M{"_id": name, "type": "template"},
		bson.M{"$set": bson.M{"content": content}},
		options.Update().SetUpsert(true),
	)
	return err
}

// DeleteSchema removes a schema from MongoDB.
func (s *MongoStore) DeleteSchema(ctx context.Context, name string) error {
	_, err := s.templatesColl.DeleteOne(ctx, bson.M{"_id": name, "type": "schema"})
	return err
}

// DeleteTemplate removes a template from MongoDB.
func (s *MongoStore) DeleteTemplate(ctx context.Context, name string) error {
	_, err := s.templatesColl.DeleteOne(ctx, bson.M{"_id": name, "type": "template"})
	return err
}
