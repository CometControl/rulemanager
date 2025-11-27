package database

import (
	"context"
	"encoding/json"
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
	schemasColl   *mongo.Collection
	templatesColl *mongo.Collection
}

type mongoRule struct {
	ID           string    `bson:"_id,omitempty"`
	TemplateName string    `bson:"templateName"`
	Parameters   bson.M    `bson:"parameters"`
	For          string    `bson:"for,omitempty"`
	CreatedAt    time.Time `bson:"createdAt"`
	UpdatedAt    time.Time `bson:"updatedAt"`
}

func toMongoRule(r *Rule) (*mongoRule, error) {
	var params bson.M
	if len(r.Parameters) > 0 {
		if err := json.Unmarshal(r.Parameters, &params); err != nil {
			return nil, err
		}
	}
	return &mongoRule{
		ID:           r.ID,
		TemplateName: r.TemplateName,
		Parameters:   params,
		For:          r.For,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}, nil
}

func fromMongoRule(mr *mongoRule) (*Rule, error) {
	params, err := json.Marshal(mr.Parameters)
	if err != nil {
		return nil, err
	}
	return &Rule{
		ID:           mr.ID,
		TemplateName: mr.TemplateName,
		Parameters:   params,
		For:          mr.For,
		CreatedAt:    mr.CreatedAt,
		UpdatedAt:    mr.UpdatedAt,
	}, nil
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
		schemasColl:   db.Collection("schemas"),
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

	mr, err := toMongoRule(rule)
	if err != nil {
		return err
	}

	_, err = s.rulesColl.InsertOne(ctx, mr)
	return err
}

// GetRule retrieves a rule by ID from MongoDB.
func (s *MongoStore) GetRule(ctx context.Context, id string) (*Rule, error) {
	var mr mongoRule
	if err := s.rulesColl.FindOne(ctx, bson.M{"_id": id}).Decode(&mr); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("rule not found")
		}
		return nil, err
	}
	return fromMongoRule(&mr)
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
	for cursor.Next(ctx) {
		var mr mongoRule
		if err := cursor.Decode(&mr); err != nil {
			return nil, err
		}
		rule, err := fromMongoRule(&mr)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// SearchRules searches for rules matching the given filter.
func (s *MongoStore) SearchRules(ctx context.Context, filter RuleFilter) ([]*Rule, error) {
	query := bson.M{}

	if filter.TemplateName != "" {
		query["templateName"] = filter.TemplateName
	}

	for key, value := range filter.Parameters {
		// Use the key exactly as provided - no automatic prefixing
		query[key] = value
	}

	cursor, err := s.rulesColl.Find(ctx, query)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []*Rule
	for cursor.Next(ctx) {
		var mr mongoRule
		if err := cursor.Decode(&mr); err != nil {
			return nil, err
		}
		rule, err := fromMongoRule(&mr)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// UpdateRule updates an existing rule in MongoDB.
func (s *MongoStore) UpdateRule(ctx context.Context, id string, rule *Rule) error {
	rule.UpdatedAt = time.Now()
	mr, err := toMongoRule(rule)
	if err != nil {
		return err
	}

	update := bson.M{
		"$set": bson.M{
			"templateName": mr.TemplateName,
			"parameters":   mr.Parameters,
			"updatedAt":    mr.UpdatedAt,
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
	ID      primitive.ObjectID `bson:"_id,omitempty"`
	Name    string             `bson:"name"`
	Content string             `bson:"content"`
}

// GetSchema retrieves a schema by name from MongoDB.
func (s *MongoStore) GetSchema(ctx context.Context, name string) (string, error) {
	var doc templateDoc
	err := s.schemasColl.FindOne(ctx, bson.M{"name": name}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", errors.New("schema not found")
		}
		return "", err
	}
	return doc.Content, nil
}

// ListSchemas retrieves all schemas from MongoDB.
func (s *MongoStore) ListSchemas(ctx context.Context) ([]*Schema, error) {
	cursor, err := s.schemasColl.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var schemas []*Schema
	for cursor.Next(ctx) {
		var doc templateDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}

		schemas = append(schemas, &Schema{
			Name:   doc.Name,
			Schema: json.RawMessage(doc.Content),
		})
	}
	return schemas, nil
}

// GetTemplate retrieves a template by name from MongoDB.
func (s *MongoStore) GetTemplate(ctx context.Context, name string) (string, error) {
	var doc templateDoc
	err := s.templatesColl.FindOne(ctx, bson.M{"name": name}).Decode(&doc)
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
	_, err := s.schemasColl.UpdateOne(
		ctx,
		bson.M{"name": name},
		bson.M{
			"$set": bson.M{
				"name":    name,
				"content": content,
			},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

// CreateTemplate saves a new template to MongoDB.
func (s *MongoStore) CreateTemplate(ctx context.Context, name, content string) error {
	_, err := s.templatesColl.UpdateOne(
		ctx,
		bson.M{"name": name},
		bson.M{
			"$set": bson.M{
				"name":    name,
				"content": content,
			},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

// DeleteSchema removes a schema from MongoDB.
func (s *MongoStore) DeleteSchema(ctx context.Context, name string) error {
	_, err := s.schemasColl.DeleteOne(ctx, bson.M{"name": name})
	return err
}

// DeleteTemplate removes a template from MongoDB.
func (s *MongoStore) DeleteTemplate(ctx context.Context, name string) error {
	_, err := s.templatesColl.DeleteOne(ctx, bson.M{"name": name})
	return err
}
