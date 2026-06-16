package nbt

import "testing"

func TestRoundTripServers(t *testing.T) {
	// Build a servers.dat-shaped tree with assorted tag types, encode, decode, compare.
	s1 := &Compound{}
	s1.Set("name", "Prod")
	s1.Set("ip", "prod.example:25565")
	s1.Set("hidden", int8(0))
	s2 := &Compound{}
	s2.Set("name", "Staging")
	s2.Set("ip", "staging.example:25565")

	root := &Compound{}
	root.Set("servers", &List{ElemType: TagCompound, Items: []any{s1, s2}})
	root.Set("someInt", int32(42))
	root.Set("someLong", int64(123456789012))
	root.Set("someDouble", float64(3.5))
	root.Set("ints", []int32{1, 2, 3})

	enc, err := Encode(root)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	dec, err := Decode(enc)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	lv, ok := dec.Get("servers")
	if !ok {
		t.Fatal("servers missing after round-trip")
	}
	lst := lv.(*List)
	if len(lst.Items) != 2 {
		t.Fatalf("want 2 servers, got %d", len(lst.Items))
	}
	got := lst.Items[0].(*Compound)
	if n, _ := got.Get("name"); n != "Prod" {
		t.Errorf("server[0].name = %v", n)
	}
	if ip, _ := got.Get("ip"); ip != "prod.example:25565" {
		t.Errorf("server[0].ip = %v", ip)
	}
	if v, _ := dec.Get("someInt"); v != int32(42) {
		t.Errorf("someInt = %v", v)
	}
	if v, _ := dec.Get("someLong"); v != int64(123456789012) {
		t.Errorf("someLong = %v", v)
	}
	if v, _ := dec.Get("someDouble"); v != float64(3.5) {
		t.Errorf("someDouble = %v", v)
	}
	if v, _ := dec.Get("ints"); len(v.([]int32)) != 3 {
		t.Errorf("ints = %v", v)
	}
}

func TestDecodeEmpty(t *testing.T) {
	// Encoding an empty root then decoding should yield an empty compound.
	enc, err := Encode(&Compound{})
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decode(enc)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec.Names) != 0 {
		t.Errorf("expected empty, got %v", dec.Names)
	}
}
