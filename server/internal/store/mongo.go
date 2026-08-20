package store

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ctxKey int

const skipMongoSync ctxKey = 1

// Mongo é o client opcional (fonte da verdade quando XVPN_MONGO_URI está set).
// O GORM em memória continua sendo a API de query dos handlers.
type mongoSync struct {
	client *mongo.Client
	db     *mongo.Database
}

func allModels() []any {
	return []any{
		&User{}, &Device{}, &InviteToken{}, &AuditLog{}, &WaitlistEntry{},
		&App{}, &AppVersion{}, &AppAsset{}, &AppAccess{},
		&PanelSettings{}, &ForgeSettings{}, &CodespaceSettings{},
		&DNSSettings{}, &DNSRecord{},
		&CloudflareAccount{}, &PublicZone{}, &PublicRecord{},
		&SocialProfile{}, &Follow{}, &SocialGroup{}, &SocialGroupMember{},
		&DirectThread{}, &DirectThreadMember{}, &Message{}, &MessageReceipt{},
		&SocialAttachment{}, &Story{}, &StoryView{},
		&SocialPost{}, &SocialPostStar{}, &SocialPostComment{},
		&ForgeOrganization{}, &OrgMember{}, &OrgTeam{}, &OrgTeamMember{},
		&Project{}, &ProjectMember{}, &ProjectStar{}, &ProtectedBranch{}, &ProjectEnv{}, &MergeRequest{}, &MergeRequestReview{}, &Issue{}, &Milestone{}, &WorkProject{}, &WorkItem{}, &CodeSpace{}, &CiJob{},
		&ForgePackage{}, &ForgePackageVersion{},
		&MeshServer{}, &ServerGroup{}, &ServerAccess{}, &BitLaunchAccount{},
		&ServiceInstance{},
		&BackupSettings{}, &BackupDestination{}, &BackupJob{},
	}
}

func collectionName(model any) string {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return strings.ToLower(t.Name())
}

// Open abre SQLite (testes/CI) ou Mongo+cache em memória (produção com URI).
func Open(path string) (*Store, error) {
	if uri := os.Getenv("XVPN_MONGO_URI"); uri != "" {
		return OpenMongo(uri, path)
	}
	return openSQLite(path)
}

func openSQLite(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("abrindo banco %q: %w", path, err)
	}
	st, err := finishOpen(db)
	if err != nil {
		return nil, err
	}
	if err := SeedIntranetDNS(st.DB); err != nil {
		return nil, fmt.Errorf("semeando DNS da intranet: %w", err)
	}
	if err := SeedXgitApp(st.DB); err != nil {
		return nil, fmt.Errorf("semeando app XGIT: %w", err)
	}
	if err := SeedXcorp(st.DB); err != nil {
		return nil, fmt.Errorf("semeando org xcorp: %w", err)
	}
	if err := SeedXcodespacesApp(st.DB); err != nil {
		return nil, fmt.Errorf("semeando app XCODESPACES: %w", err)
	}
	if err := SeedBackupSettings(st.DB); err != nil {
		return nil, fmt.Errorf("semeando backups: %w", err)
	}
	return st, nil
}

// OpenMongo usa mongod como fonte da verdade e SQLite em memória como
// cache GORM (handlers existentes). path é só para import one-shot se o
// Mongo estiver vazio e ainda existir xvpn.db.
func OpenMongo(uri, sqlitePath string) (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}
	mdb := client.Database("xvpn")

	mem, err := gorm.Open(sqlite.Open("file:xvpn-mongo-cache?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("cache sqlite: %w", err)
	}
	st, err := finishOpen(mem)
	if err != nil {
		return nil, err
	}
	st.mongo = &mongoSync{client: client, db: mdb}

	empty, err := mongoIsEmpty(ctx, mdb)
	if err != nil {
		return nil, err
	}
	if empty && sqlitePath != "" {
		if _, err := os.Stat(sqlitePath); err == nil {
			if err := ImportSQLiteToMongo(sqlitePath, uri); err != nil {
				return nil, fmt.Errorf("import sqlite→mongo: %w", err)
			}
		}
	}
	if err := st.hydrateFromMongo(ctx); err != nil {
		return nil, fmt.Errorf("hydrate mongo→cache: %w", err)
	}
	st.registerMongoCallbacks()
	if err := SeedIntranetDNS(st.DB); err != nil {
		return nil, fmt.Errorf("semeando DNS da intranet: %w", err)
	}
	if err := SeedXgitApp(st.DB); err != nil {
		return nil, fmt.Errorf("semeando app XGIT: %w", err)
	}
	if err := SeedXcorp(st.DB); err != nil {
		return nil, fmt.Errorf("semeando org xcorp: %w", err)
	}
	if err := SeedXcodespacesApp(st.DB); err != nil {
		return nil, fmt.Errorf("semeando app XCODESPACES: %w", err)
	}
	if err := SeedBackupSettings(st.DB); err != nil {
		return nil, fmt.Errorf("semeando backups: %w", err)
	}
	return st, nil
}

func mongoIsEmpty(ctx context.Context, db *mongo.Database) (bool, error) {
	n, err := db.Collection("user").CountDocuments(ctx, bson.M{})
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

func finishOpen(db *gorm.DB) (*Store, error) {
	needsRoleBackfill := !db.Migrator().HasColumn(&User{}, "role")
	if err := db.AutoMigrate(allModels()...); err != nil {
		return nil, fmt.Errorf("migrando schema: %w", err)
	}
	if needsRoleBackfill {
		if err := backfillInitialRoles(db); err != nil {
			return nil, fmt.Errorf("migrando papéis (Fase 10): %w", err)
		}
	}
	return &Store{DB: db}, nil
}

func (s *Store) hydrateFromMongo(ctx context.Context) error {
	skip := context.WithValue(ctx, skipMongoSync, true)
	for _, model := range allModels() {
		coll := s.mongo.db.Collection(collectionName(model))
		cur, err := coll.Find(ctx, bson.M{})
		if err != nil {
			return err
		}
		t := reflect.TypeOf(model)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		for cur.Next(ctx) {
			inst := reflect.New(t).Interface()
			if err := cur.Decode(inst); err != nil {
				_ = cur.Close(ctx)
				return err
			}
			applyBSONID(inst, cur.Current)
			if err := s.DB.WithContext(skip).Create(inst).Error; err != nil {
				_ = cur.Close(ctx)
				return fmt.Errorf("hydrate %s: %w", t.Name(), err)
			}
		}
		if err := cur.Close(ctx); err != nil {
			return err
		}
	}
	return nil
}

func applyBSONID(model any, raw bson.Raw) {
	var wrap struct {
		ID uint `bson:"_id"`
	}
	if err := bson.Unmarshal(raw, &wrap); err != nil || wrap.ID == 0 {
		return
	}
	rv := reflect.ValueOf(model)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	f := rv.FieldByName("ID")
	if f.IsValid() && f.CanSet() && f.Kind() == reflect.Uint {
		f.SetUint(uint64(wrap.ID))
	}
}

func (s *Store) registerMongoCallbacks() {
	_ = s.DB.Callback().Create().After("gorm:after_create").Register("xvpn:mongo_create", s.cbMongoUpsert)
	_ = s.DB.Callback().Update().After("gorm:after_update").Register("xvpn:mongo_update", s.cbMongoUpsert)
	_ = s.DB.Callback().Delete().After("gorm:after_delete").Register("xvpn:mongo_delete", s.cbMongoDelete)
}

func (s *Store) cbMongoUpsert(db *gorm.DB) {
	if s.mongo == nil || db.Statement == nil || db.Statement.Dest == nil {
		return
	}
	if db.Statement.Context != nil && db.Statement.Context.Value(skipMongoSync) != nil {
		return
	}
	doc := structToBSON(db.Statement.Dest)
	id, ok := doc["_id"]
	if !ok {
		return
	}
	name := collectionName(db.Statement.Dest)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.mongo.db.Collection(name).ReplaceOne(ctx, bson.M{"_id": id}, doc, options.Replace().SetUpsert(true))
}

func (s *Store) cbMongoDelete(db *gorm.DB) {
	if s.mongo == nil || db.Statement == nil || db.Statement.Dest == nil {
		return
	}
	if db.Statement.Context != nil && db.Statement.Context.Value(skipMongoSync) != nil {
		return
	}
	doc := structToBSON(db.Statement.Dest)
	id, ok := doc["_id"]
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.mongo.db.Collection(collectionName(db.Statement.Dest)).DeleteOne(ctx, bson.M{"_id": id})
}

func structToBSON(v any) bson.M {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return bson.M{}
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Slice {
		if rv.Len() == 1 {
			return structToBSON(rv.Index(0).Interface())
		}
		return bson.M{}
	}
	if rv.Kind() != reflect.Struct {
		return bson.M{}
	}
	rt := rv.Type()
	out := bson.M{}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		if skipAssoc(f.Type) {
			continue
		}
		val := rv.Field(i).Interface()
		if f.Name == "ID" {
			out["_id"] = val
			continue
		}
		out[f.Name] = val
	}
	return out
}

func skipAssoc(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice {
		return true
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	if t == reflect.TypeOf(time.Time{}) {
		return false
	}
	switch t.Name() {
	case "User", "App", "AppVersion", "Device", "InviteToken":
		return true
	default:
		return false
	}
}

// ImportSQLiteToMongo copia um xvpn.db para o Mongo (one-shot da Fase 28).
func ImportSQLiteToMongo(sqlitePath, mongoURI string) error {
	src, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return err
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	mdb := client.Database("xvpn")
	for _, model := range allModels() {
		slice := reflect.New(reflect.SliceOf(reflect.TypeOf(model).Elem())).Interface()
		if err := src.Find(slice).Error; err != nil {
			return fmt.Errorf("lendo %s: %w", collectionName(model), err)
		}
		rv := reflect.ValueOf(slice).Elem()
		if rv.Len() == 0 {
			continue
		}
		docs := make([]any, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			docs = append(docs, structToBSON(rv.Index(i).Addr().Interface()))
		}
		coll := mdb.Collection(collectionName(model))
		if _, err := coll.InsertMany(ctx, docs); err != nil {
			return fmt.Errorf("gravando %s: %w", collectionName(model), err)
		}
	}
	return nil
}
