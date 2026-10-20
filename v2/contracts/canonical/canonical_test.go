package canonical

import "testing"

func TestCanonical(t *testing.T) {
	got, err := FromJSON([]byte(`{ "b": 1, "a": [true, null, "x<y&z"], "c": {"z": 2, "y": 10000000000} }`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":[true,null,"x<y&z"],"b":1,"c":{"y":10000000000,"z":2}}`
	if string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestMarshalWithout(t *testing.T) {
	got, err := MarshalWithout(map[string]any{"a": 1, "signature": "x"}, "signature")
	if err != nil || string(got) != `{"a":1}` {
		t.Fatalf("%s %v", got, err)
	}
}
