package jsonpath

import (
	"log"
	"testing"
)

func TestJsonPath(t *testing.T) {
	j, err := NewJson([]byte(`
        {
            "a": [
                {
                    "b": 1
                },
                {
                    "b": 2
                }
            ]
        }
    `))
	if err != nil {
		t.Error(err)
	}

	{
		a, err := j.Query("a[0].b")
		if err != nil {
			t.Error(err)
			return
		}
		if v, ok := a.(float64); !ok || int(v) != 1 {
			t.Error("expect a[0].b == 1, real", v)
			return
		}
		log.Println("----------------")
	}

	{
		a, err := j.Query("a[:].b")
		if err != nil {
			t.Error(err)
			return
		}
		b, ok := a.([]interface{})
		if !ok {
			t.Error()
			return
		}
		if v, ok := b[0].(float64); !ok || int(v) != 1 {
			t.Error()
			return
		}
		if v, ok := b[1].(float64); !ok || int(v) != 2 {
			t.Error()
			return
		}
		log.Println("----------------")
	}

	{
		a, err := j.Query("a.(b=1)[0].b")
		if err != nil {
			t.Error(err)
			return
		}
		t.Log(a)
		if v, ok := a.(float64); !ok || int(v) != 1 {
			t.Error(v)
			return
		}
	}
}
