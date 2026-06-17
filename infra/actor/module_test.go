package actor

import (
	"context"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/godyy/ggskit/base/db/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// testModelDirty 脏数据模型.
type testModelDirty struct {
	actor   ActorWithModel // 关联ActorWithModel.
	dirties bson.M         // 脏数据
	all     bool           // 是否全脏位.
}

// newTestModelDirty 构造脏数据模型.
func newTestModelDirty() *testModelDirty {
	return &testModelDirty{}
}

// SetDirty 设置脏数据.
func (md *testModelDirty) SetDirty(key string, value any) {
	if md.dirties == nil {
		md.dirties = make(bson.M)
	}
	md.dirties[key] = value
}

// SetDirtyAll 设置全脏位.
func (md *testModelDirty) SetDirtyAll() {
	md.all = true
}

// IsDirty 是否有脏数据.
func (md *testModelDirty) IsDirty() (dirty bool, all bool) {
	all = md.all
	dirty = all || len(md.dirties) > 0
	return
}

// ClearDirty 清除脏数据.
func (md *testModelDirty) ClearDirty() {
	md.dirties = nil
	md.all = false
}

// MarshalBSONDirty 序列化脏数据.
func (md *testModelDirty) MarshalBSONDirty() ([]byte, error) {
	return bson.Marshal(md.dirties)
}

func (md *testModelDirty) Release() {
	md.actor = nil
	md.dirties = nil
}

type testModel struct {
	mr              *ModuleRegistry
	ID              int64    `bson:"id"`
	Name            string   `bson:"name"`
	Modules         *Modules `bson:"modules"`
	*testModelDirty `bson:"-"`
}

func newTestModel(mr *ModuleRegistry) *testModel {
	m := &testModel{
		mr:             mr,
		testModelDirty: newTestModelDirty(),
	}
	m.Modules = NewModules(mr)
	return m
}

func (m *testModel) ModuleRegistry() *ModuleRegistry {
	return m.mr
}

func (m *testModel) SetModuleDirty(key ModuleKey) {
	if module := m.Modules.GetModule(key, false); module != nil {
		m.testModelDirty.SetDirty("modules."+module.ModuleKey(), module)
	}
}

func (m *testModel) GetModule(key ModuleKey, autoCreate bool) Module {
	return m.Modules.GetModule(key, autoCreate)
}

func (m *testModel) GetHashKey() any { return m.ID }

func (m *testModel) GetCollection() string { return "test_models" }

func (m *testModel) GetFilter() any {
	return bson.M{"id": m.ID}
}

func (m *testModel) Release() {
	m.testModelDirty.ClearDirty()
	m.Modules.Release()
	m.mr = nil
}

type testModuleA struct {
	Value string
}

func (m *testModuleA) OnInit() {}

func (m *testModuleA) ModuleKey() string { return "A" }

type testModuleB struct {
	Value string
}

func (m *testModuleB) OnInit() {}

func (m *testModuleB) ModuleKey() string { return "B" }

type testModuleKeySA struct{}

func (k testModuleKeySA) ModuleKey() string { return "SA" }

type testSA struct {
	Value string
}

type testModuleSA = ModuleSingle[*string, testModuleKeySA]

func testPValue[V any](v V) *V {
	return &v
}

func TestModulesCodec(t *testing.T) {
	registry := NewModuleRegistry()
	RegisterModule[*testModuleB](registry)
	RegisterModule[*testModuleA](registry)
	RegisterModule[*testModuleSA](registry)

	modelSrc := newTestModel(registry)
	modelSrc.ID = 1
	modelSrc.Name = "test"
	modelSrc.Modules.InitAllModules()
	GetModule[*testModuleA](modelSrc, false).Value = "this is module A"
	GetModule[*testModuleB](modelSrc, false).Value = "this is module B"
	GetModule[*testModuleSA](modelSrc, false).Set(testPValue("123"))
	modelSrcBSON, err := bson.Marshal(modelSrc)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(hex.EncodeToString(modelSrcBSON))

	modelDst := newTestModel(registry)
	if err := bson.Unmarshal(modelSrcBSON, modelDst); err != nil {
		t.Fatal(err)
	}

	modelDstBSON, err := bson.Marshal(modelDst)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(hex.EncodeToString(modelDstBSON))

	if !reflect.DeepEqual(modelDstBSON, modelSrcBSON) {
		t.Fatalf("dst:%+v not equal src:%+v", modelDst, modelSrc)
	}

	GetModule[*testModuleA](modelDst, false).Value = "this is module AA"
	modelDst.SetModuleDirty(GetModule[*testModuleA](modelDst, false))
	GetModule[*testModuleB](modelDst, false).Value = "this is module BB"
	modelDst.SetModuleDirty(GetModule[*testModuleB](modelDst, false))
	if dirty, _ := modelDst.testModelDirty.IsDirty(); !dirty {
		t.Fatal("actorDst.DirtyMgr not dirty")
	}

	GetModule[*testModuleA](modelDst, false).Value = "this is module AAA"
	modelDst.SetModuleDirty(GetModule[*testModuleA](modelDst, false))
	GetModule[*testModuleB](modelDst, false).Value = "this is module BBB"
	modelDst.SetModuleDirty(GetModule[*testModuleB](modelDst, false))
	modelDstBSON, err = modelDst.MarshalBSONDirty()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(hex.EncodeToString(modelDstBSON))
}

func TestModulesDirty(t *testing.T) {
	cli, err := mongo.Connect(&mongo.Config{
		URI: "mongodb://localhost:27017/?readPreference=primary",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Disconnect(context.Background())
	db := cli.Database("test")
	coll := db.Collection("test_models")

	registry := NewModuleRegistry()
	RegisterModule[*testModuleB](registry)
	RegisterModule[*testModuleA](registry)

	model := newTestModel(registry)
	model.ID = 1
	model.Name = "test"
	model.Modules.InitAllModules()
	if _, err := coll.InsertOne(context.Background(), model); err != nil {
		t.Fatal(err)
	}

	GetModule[*testModuleA](model, false).Value = "this is module A"
	model.SetModuleDirty(GetModule[*testModuleA](model, false))
	// GetModule[*testModuleB](model).Value = "this is module B"
	// GetModule[*testModuleB](model).SetDirty()
	modelDirtyBSON, err := model.MarshalBSONDirty()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(hex.EncodeToString(modelDirtyBSON))

	if _, err := coll.UpdateOne(context.Background(),
		bson.M{"id": model.ID},
		bson.M{"$set": bson.Raw(modelDirtyBSON)},
	); err != nil {
		t.Fatal(err)
	}

}
