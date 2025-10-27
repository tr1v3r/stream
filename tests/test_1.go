package tests

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/tr1v3r/stream"
)

// - Q1: 输入 employees，返回 年龄 >22岁 的所有员工，年龄总和
func Question1Sub1(employees []*Employee) int64 {
	return stream.SliceOf[*Employee](employees...).
		Filter(func(e *Employee) bool { // Filter 过滤年龄
			return e.Age > 22
		}).ReduceWith(int64(0), func(acc any, e *Employee) any { // ReduceWith 累加年龄
		return acc.(int64) + int64(e.Age)
	}).(int64)
}

// - Q2: - 输入 employees，返回 id 最小的十个员工，按 id 升序排序
func Question1Sub2(employees []*Employee) []*Employee {
	return stream.SliceOf[*Employee](employees...).Sort(func(left, right *Employee) int {
		if left.ID < right.ID {
			return -1
		}
		if left.ID > right.ID {
			return -1
		}
		return 0
	}).Limit(10).ToSlice()
}

// - Q3: - 输入 employees，对于没有手机号为0的数据，随机填写一个
func Question1Sub3(employees []*Employee) []*Employee {
	// Create a seeded random source for this function
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return stream.SliceOf[*Employee](employees...).
		Convert(func(e *Employee) any {
			if e.Phone == "" {
				e.Phone = fmt.Sprintf("%10d", r.Int())
			}
			return e
			// }).Collect(stream.To[any, *Employee](func(t any) *Employee { return t.(*Employee) })).([]*Employee)
		}).Collect(stream.AnyTo[*Employee]()).([]*Employee)
}

// - Q4: - 输入 employees ，返回一个map[int][]int，其中 key 为 员工年龄 Age，value 为该年龄段员工ID
func Question1Sub4(employees []*Employee) map[int][]int64 {
	return stream.SliceOf(employees...).
		ReduceWith(make(map[int][]int64), func(acc any, e *Employee) any {
			m := acc.(map[int][]int64)
			m[e.Age] = append(m[e.Age], e.ID)
			return m
		}).(map[int][]int64)
}

// - Q1: 计算一个 string 中小写字母的个数
func Question2Sub1(str string) int64 {
	return stream.SliceOf[byte]([]byte(str)...).ReduceWith(int64(0), func(acc any, c byte) any {
		n := acc.(int64)
		if 97 <= c && c <= 122 {
			n++
		}
		return n
	}).(int64)
}

// - Q2: 找出 []string 中，包含小写字母最多的字符串
func Question2Sub2(list []string) string {
	var max = int64(0)
	return stream.SliceOf[string](list...).ReduceWith("", func(acc any, s string) any {
		if n := Question2Sub1(s); n > max {
			max = n
			return s
		}
		return acc.(string)
	}).(string)
}

// - Q1: 输入一个整数 int，字符串string。将这个字符串重复n遍返回
func Question3Sub1(str string, n int) string {
	// intRange n times, Foreach append
	return stream.RepeatN(str, int64(n)).ReduceWith("", func(acc any, s string) any {
		return acc.(string) + s
	}).(string)
}
