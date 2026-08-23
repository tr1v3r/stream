package tests

import "testing"

func Test_Question1(t *testing.T) {
	t.Logf("question 1-1: %d", Question1Sub1([]*Employee{
		{Age: 18},
		{Age: 19},
		{Age: 20},
		{Age: 21},
		{Age: 22},
		{Age: 23},
		{Age: 24},
		{Age: 25},
		{Age: 26},
	}))

	results := Question1Sub3([]*Employee{
		{Age: 18},
		{Age: 19},
		{Age: 20},
		{Age: 21},
		{Age: 22},
		{Age: 23},
		{Age: 24},
		{Age: 25},
		{Age: 26},
	})
	t.Logf("question 1-3: %+v", results)
}

func Test_Question3(t *testing.T) {
	t.Logf("question 3-1: %s", Question3Sub1("hello ", 4))
}

func Test_Question1Sub2(t *testing.T) {
	emps := make([]*Employee, 15)
	for i := range emps {
		emps[i] = &Employee{ID: int64(15 - i)} // IDs 15..1, shuffled input
	}
	got := Question1Sub2(emps)
	if len(got) != 10 {
		t.Fatalf("expected 10 employees, got %d", len(got))
	}
	for i, e := range got {
		if e.ID != int64(i+1) {
			t.Fatalf("expected ascending IDs, position %d has id %d", i, e.ID)
		}
	}
}
