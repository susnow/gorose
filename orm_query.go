package gorose

import (
	"fmt"
	"github.com/gohouse/t"
	"math"
	"reflect"
)

// Get : select more rows , relation limit set
func (dba *Orm) Get() error {
	// 构建sql
	sqlStr, args, err := dba.BuildSql()
	if err != nil {
		return err
	}

	// 执行查询
	return dba.ISession.Query(sqlStr, args...)
}

// Count : select count rows
func (dba *Orm) Count(args ...string) (int64, error) {
	fields := "*"
	if len(args) > 0 {
		fields = args[0]
	}
	count, err := dba._unionBuild("count", fields)
	if count == nil {
		count = int64(0)
	}
	return count.(int64), err
}

// Sum : select sum field
func (dba *Orm) Sum(sum string) (interface{}, error) {
	return dba._unionBuild("sum", sum)
}

// Avg : select avg field
func (dba *Orm) Avg(avg string) (interface{}, error) {
	return dba._unionBuild("avg", avg)
}

// Max : select max field
func (dba *Orm) Max(max string) (interface{}, error) {
	return dba._unionBuild("max", max)
}

// Min : select min field
func (dba *Orm) Min(min string) (interface{}, error) {
	return dba._unionBuild("min", min)
}

// _unionBuild : build union select real
func (dba *Orm) _unionBuild(union, field string) (interface{}, error) {
	var tmp interface{}

	dba.union = union + "(" + field + ") as " + union
	// 缓存fields字段,暂时由union占用
	fieldsTmp := dba.fields
	dba.fields = []string{dba.union}
	dba.ISession.SetUnion(true)

	// 构建sql
	sqls, args, err := dba.BuildSql()
	if err != nil {
		return tmp, err
	}

	// 执行查询
	err = dba.ISession.Query(sqls, args...)
	if err != nil {
		return tmp, err
	}

	// 重置union, 防止复用的时候感染
	dba.union = ""
	// 返还fields
	dba.fields = fieldsTmp

	// 语法糖获取union值
	if dba.ISession.GetUnion() != nil {
		tmp = dba.ISession.GetUnion()
		dba.ISession.SetUnion(nil)
	}

	return tmp, nil
}

// Get : select more rows , relation limit set
func (dba *Orm) Value(field string) (v t.T, err error) {
	dba.Limit(1)
	err = dba.Get()
	if err != nil {
		return
	}
	var binder = dba.ISession.GetBinder()
	switch binder.GetBindType() {
	case OBJECT_MAP, OBJECT_MAP_SLICE, OBJECT_MAP_SLICE_T, OBJECT_MAP_T:
		v = t.New(binder.GetBindResult().MapIndex(reflect.ValueOf(field)).Interface())
	case OBJECT_STRUCT, OBJECT_STRUCT_SLICE:
		bindResult := reflect.Indirect(binder.GetBindResult())
		v = dba._valueFromStruct(bindResult, field)
	}
	return
}
func (dba *Orm) _valueFromStruct(bindResult reflect.Value, field string) (v t.T) {
	ostype := bindResult.Type()
	for i := 0; i < ostype.NumField(); i++ {
		tag := ostype.Field(i).Tag.Get(TAGNAME)
		if tag == field || ostype.Field(i).Name == field {
			v = t.New(bindResult.FieldByName(ostype.Field(i).Name))
		}
	}
	return
}
func (dba *Orm) Pluck(field string, fieldKey ...string) (v t.T, err error) {
	err = dba.Get()
	if err != nil {
		return
	}
	var binder = dba.ISession.GetBinder()
	var resMap = make(t.MapInterface, 0)
	var resSlice = t.Slice{}
	switch binder.GetBindType() {
	case OBJECT_MAP, OBJECT_MAP_T, OBJECT_STRUCT: // row
		var key, val t.T
		if len(fieldKey) > 0 {
			key, err = dba.Value(fieldKey[0])
			if err != nil {
				return
			}
			val, err = dba.Value(field)
			if err != nil {
				return
			}
			v = t.New(t.Map{key: val})
		} else {
			v, err = dba.Value(field)
			if err != nil {
				return
			}
		}
		return
	case OBJECT_MAP_SLICE, OBJECT_MAP_SLICE_T:
		for _, item := range t.New(binder.GetBindResultSlice().Interface()).Slice() {
			val := item.MapInterface()
			if len(fieldKey) > 0 {
				resMap[val[fieldKey[0]].Interface()] = val[field]
			} else {
				resSlice = append(resSlice, val[field])
			}
		}
	case OBJECT_STRUCT_SLICE: // rows
		var brs = binder.GetBindResultSlice()
		for i := 0; i < brs.Len(); i++ {
			val := reflect.Indirect(brs.Index(i))
			if len(fieldKey) > 0 {
				mapkey := dba._valueFromStruct(val, fieldKey[0])
				mapVal := dba._valueFromStruct(val, field)
				resMap[mapkey.Interface()] = mapVal
			} else {
				resSlice = append(resSlice, dba._valueFromStruct(val, field))
			}
		}
	}
	if len(fieldKey) > 0 {
		v = t.New(t.New(resMap).Map())
	} else {
		v = t.New(resSlice)
	}
	return
}

// Chunk : select chunk more data to piceses block
func (dba *Orm) Chunk(limit int, callback func(interface{}) error) (err error) {
	return nil
	var page = 0
	dba.Limit(limit)
	count, _ := dba.Count()
	for count > 0 {
		var binder = dba.ISession.GetBinder()

		// 设置指定的limit, offset
		_ = dba.Offset(dba.offset + page*limit).Get()
		fmt.Println(dba.LastSql())
		var result = binder.GetBindResultSlice()

		if result.Len() == 0 {
			break
		}

		if err = callback(result.Interface()); err != nil {
			break
		}

		page++
		count = count - int64(limit)

		var bs = binder.GetBindResultSlice()
		binder.SetBindResultSlice(reflect.MakeSlice(bs.Type(),bs.Len(),bs.Cap()))
		binder.SetBindOrigin(nil)
	}
	return
}

// Paginate 自动分页
// @param limit 每页展示数量
// @param current_page 当前第几页, 从1开始
// 以下是laravel的Paginate返回示例
//{
//	"total": 50,
//	"per_page": 15,
//	"current_page": 1,
//	"last_page": 4,
//	"first_page_url": "http://laravel.app?page=1",
//	"last_page_url": "http://laravel.app?page=4",
//	"next_page_url": "http://laravel.app?page=2",
//	"prev_page_url": null,
//	"path": "http://laravel.app",
//	"from": 1,
//	"to": 15,
//	"data":[
//		{
//		// Result Object
//		},
//		{
//		// Result Object
//		}
//	]
//}
func (dba *Orm) Paginate(limit, current_page int) (res Data, err error) {
	// 防止limit干扰
	dba.limit = 0
	// 统计总量
	count, err := dba.Count()
	// 获取结果
	var bind = Data{}
	err = dba.Table(&bind).Limit(limit).Get()
	if err != nil {
		return
	}
	var last_page = int(math.Ceil(float64(count) / float64(limit)))
	var next_page = current_page + 1
	var prev_page = current_page - 1
	res = Data{
		"total":          count,
		"per_page":       limit,
		"current_page":   current_page,
		"last_page":      last_page,
		"first_page_url": 1,
		"last_page_url":  last_page,
		"next_page_url":  If(next_page > last_page, nil, next_page),
		"prev_page_url":  If(prev_page < 1, nil, prev_page),
		"data":           dba.GetBinder().GetBindResultSlice(),
	}

	return
}
