package mongobd

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Op 操作符
type Op interface {
	base() *opBase
	exec(*mongo.Client) // 执行操作符

	Err() error         // 获取错误信息
	Callback() func(Op) // 获取回调函数
}

// opDone 操作符完成
// 操作符执行完成后, 调用此函数将操作符发送到完成通道.
func opDone(op Op, err error) {
	base := op.base()
	base.err = err
	base.done <- op
}

// opBase 操作符基础封装.
type opBase struct {
	DB   string
	Coll string

	ctx      context.Context
	done     chan Op
	callback func(Op)
	err      error
}

func (o *opBase) base() *opBase {
	return o
}

func (o *opBase) Err() error {
	return o.err
}

func (o *opBase) Callback() func(Op) {
	return o.callback
}

func NewOp[O any](db, coll string) *O {
	return NewOpWithContext[O](context.Background(), db, coll)
}

func NewOpWithContext[O any](ctx context.Context, db, coll string) *O {
	if ctx == nil {
		ctx = context.Background()
	}
	var op *O
	if _, ok := any(op).(Op); !ok {
		panic(fmt.Errorf("%T not implement Op", op))
	}
	op = new(O)
	base := any(op).(Op).base()
	base.ctx = ctx
	base.DB = db
	base.Coll = coll
	return op
}

// OpLoad 加载操作符
// 从数据库中加载符合条件的文档到指定的目标数据结构中.
type OpLoad struct {
	opBase
	Filter     any  // 查询过滤条件
	Projection any  // 投影字段
	Primary    bool // 是否使用主节点读取
	Target     any  // 目标数据结构
}

func (o *OpLoad) SetFilter(filter any) *OpLoad {
	o.Filter = filter
	return o
}

func (o *OpLoad) SetProjection(projection any) *OpLoad {
	o.Projection = projection
	return o
}

func (o *OpLoad) SetPrimary(primary bool) *OpLoad {
	o.Primary = primary
	return o
}

func (o *OpLoad) SetTarget(target any) *OpLoad {
	o.Target = target
	return o
}

// exec 加载操作符执行逻辑.
func (o *OpLoad) exec(client *mongo.Client) {
	collOpts := options.Collection().SetReadConcern(readconcern.Majority())
	if o.Primary {
		collOpts.SetReadPreference(readpref.Primary())
	} else {
		collOpts.SetReadPreference(readpref.Secondary())
	}
	coll := client.Database(o.DB).Collection(o.Coll, collOpts)

	findOneOpts := options.FindOne()
	if o.Projection != nil {
		findOneOpts.SetProjection(o.Projection)
	}
	err := coll.FindOne(o.ctx, o.Filter, findOneOpts).Decode(o.Target)
	opDone(o, err)
}

// OpUpdate 更新操作符
// 更新数据库中符合条件的文档.
type OpUpdate struct {
	opBase
	Filter any  // 查询过滤条件
	Update any  // 更新操作符
	Upsert bool // 是否插入新文档
}

func (o *OpUpdate) SetFilter(filter any) *OpUpdate {
	o.Filter = filter
	return o
}

func (o *OpUpdate) SetUpdate(update any) *OpUpdate {
	o.Update = update
	return o
}

func (o *OpUpdate) SetUpsert(upsert bool) *OpUpdate {
	o.Upsert = upsert
	return o
}

// exec 更新操作符执行逻辑.
func (o *OpUpdate) exec(client *mongo.Client) {
	coll := client.Database(o.DB).Collection(o.Coll)

	updateOneOpts := options.UpdateOne()
	if o.Upsert {
		updateOneOpts.SetUpsert(true)
	}
	_, err := coll.UpdateOne(o.ctx, o.Filter, bson.M{"$set": o.Update}, updateOneOpts)
	opDone(o, err)
}
