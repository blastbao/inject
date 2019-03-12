// Package inject provides utilities for mapping and injecting dependencies in various ways.
package inject

import (
	"fmt"
	"reflect"
)

// Injector represents an interface for mapping and injecting dependencies into structs
// and function arguments.
type Injector interface {
	Applicator
	Invoker
	TypeMapper
	// SetParent sets the parent of the injector. If the injector cannot find a
	// dependency in its Type map it will check its parent before returning an
	// error.
	SetParent(Injector)
}

// Applicator represents an interface for mapping dependencies to a struct.
type Applicator interface {
	// Maps dependencies in the Type map to each field in the struct
	// that is tagged with 'inject'. Returns an error if the injection
	// fails.
	Apply(interface{}) error
}

// Invoker represents an interface for calling functions via reflection.
type Invoker interface {
	// Invoke attempts to call the interface{} provided as a function,
	// providing dependencies for function arguments based on Type. Returns
	// a slice of reflect.Value representing the returned values of the function.
	// Returns an error if the injection fails.
	Invoke(interface{}) ([]reflect.Value, error)
}

// TypeMapper represents an interface for mapping interface{} values based on type.
type TypeMapper interface {
	// Maps the interface{} value based on its immediate type from reflect.TypeOf.
	Map(interface{}) TypeMapper
	// Maps the interface{} value based on the pointer of an Interface provided.
	// This is really only useful for mapping a value as an interface, as interfaces
	// cannot at this time be referenced directly without a pointer.
	MapTo(interface{}, interface{}) TypeMapper
	// Provides a possibility to directly insert a mapping based on type and value.
	// This makes it possible to directly map type arguments not possible to instantiate
	// with reflect like unidirectional channels.
	Set(reflect.Type, reflect.Value) TypeMapper
	// Returns the Value that is mapped to the current type. Returns a zeroed Value if
	// the Type has not been mapped.
	Get(reflect.Type) reflect.Value
}

type injector struct {
	values map[reflect.Type] reflect.Value
	parent Injector
}

// InterfaceOf dereferences a pointer to an Interface type.
// It panics if value is not an pointer to an interface.

// 从 interface 的指针中获取元素类型，以(*SpecialString)(nil)为例：
//
// 	type SpecialString interface{}
// 	func main() {   
//   	fmt.Println(inject.InterfaceOf((*interface{})(nil)))      
//   	fmt.Println(inject.InterfaceOf((*SpecialString)(nil)))
// 	}
// 输出：
// 	interface {}
// 	main.SpecialString
//
// 可见，指向某接口的空指针 (*SpecialString)(nil) ，虽然 data 部分是nil，但是却可以取出它的type。

func InterfaceOf(value interface{}) reflect.Type {
	//获取 value 的类型 t
	t := reflect.TypeOf(value)           //t是*main.SpecialString，t.Kind()是ptr，t.Elem()是main.SpecialString
	//循环解引用（指向指针的指针），得到实际元素类型t
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	//这块对类型的约束是：t必须是指向interface{}的指针，如果不是Interface{}类型，报panic
	if t.Kind() != reflect.Interface {
		panic("Called inject.InterfaceOf with a value that is not a pointer to an interface. (*MyInterface)(nil)")
	}
	//返回类型t，也即 (*SpecialString)(nil) 的元素原始类型 main.SpecialString
	return t
}








// New returns a new Injector.
func New() Injector {
	return &injector{
		values: make(map[reflect.Type]reflect.Value),
	}
}

// Invoke attempts to call the interface{} provided as a function,
// providing dependencies for function arguments based on Type.
// Returns a slice of reflect.Value representing the returned values of the function.
// Returns an error if the injection fails.
// It panics if f is not a function




// 将函数的值从空接口中反射出来，然后使用 reflect.Call 来传递参数并调用它。
// 参数个数从 t.NumIn() 获取，循环遍历参数类型，再通过 Get 方法从 values map[reflect.Type]reflect.Value 获取对应类型的具体参数对象。

func (inj *injector) Invoke(f interface{}) ([]reflect.Value, error) {
	t := reflect.TypeOf(f)

	var in = make([]reflect.Value, t.NumIn()) //Panic if t is not kind of Func
	for i := 0; i < t.NumIn(); i++ {
		argType := t.In(i)
		val := inj.Get(argType)
		if !val.IsValid() {
			return nil, fmt.Errorf("Value not found for type %v", argType)
		}

		in[i] = val
	}

	return reflect.ValueOf(f).Call(in), nil
}

// Maps dependencies in the Type map to each field in the struct
// that is tagged with 'inject'.
// Returns an error if the injection fails.
func (inj *injector) Apply(val interface{}) error {
	//获取val的值
	v := reflect.ValueOf(val)
	//解引用
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	//只能对结构体进行依赖对象的注入
	if v.Kind() != reflect.Struct {
		return nil // Should not panic here ?
	}
	//获取val的类型
	t := v.Type()
	//逐个遍历val的字段f...
	for i := 0; i < v.NumField(); i++ {
		// reflect.value用来设置字段值，要求f.CanSet()为true
		f := v.Field(i)
		// reflect.type用来获取字段tags，依赖注入的标签名是"inject"
		structField := t.Field(i)
		if f.CanSet() && (structField.Tag == "inject" || structField.Tag.Get("inject") != "") {
			//获取字段f的类型，该类型用来从注册容器中查找依赖对象
			ft := f.Type()
			//从容器中取字段f的类型ft对应的依赖对象
			v := inj.Get(ft)
			//如果该依赖对象是无效的（空指针），报错
			if !v.IsValid() {
				return fmt.Errorf("Value not found for type %v", ft)
			}
			//执行注入
			f.Set(v)
		}
	}
	return nil
}

// Maps the concrete value of val to its dynamic type using reflect.TypeOf,
// It returns the TypeMapper registered in.
func (i *injector) Map(val interface{}) TypeMapper {
	i.values[reflect.TypeOf(val)] = reflect.ValueOf(val)
	return i
}





func (i *injector) MapTo(val interface{}, ifacePtr interface{}) TypeMapper {
	i.values[InterfaceOf(ifacePtr)] = reflect.ValueOf(val)
	return i
}







// Maps the given reflect.Type to the given reflect.Value and returns
// the Typemapper the mapping has been registered in.
func (i *injector) Set(typ reflect.Type, val reflect.Value) TypeMapper {
	i.values[typ] = val
	return i
}

func (i *injector) Get(t reflect.Type) reflect.Value {

	//根据t获取依赖对象val
	val := i.values[t]
	//如果val不是nil，直接返回该依赖对象
	if val.IsValid() {
		return val
	}

	// 如果val是nil，则判断：若t是interface类型（不是struct类型），
	// 且容器中有依赖对象实现了该interface，就返回第一个实现该interface的依赖对象。

	// no concrete types found, try to find implementors
	// if t is an interface
	if t.Kind() == reflect.Interface {
		for k, v := range i.values {
			if k.Implements(t) {
				val = v
				break
			}
		}
	}

	// Still no type found, try to look it up on the parent
	if !val.IsValid() && i.parent != nil {
		val = i.parent.Get(t) //递归回溯
	}

	return val

}

func (i *injector) SetParent(parent Injector) {
	i.parent = parent
}
