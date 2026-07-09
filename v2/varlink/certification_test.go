package varlink

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strconv"
	"testing"
)

type certificationInterface struct {
	Foo  *[]*map[string]string `json:"foo"`
	Anon struct {
		Foo bool `json:"foo"`
		Bar bool `json:"bar"`
	} `json:"anon"`
}

type certificationMyType struct {
	Object              json.RawMessage        `json:"object"`
	Enum                string                 `json:"enum"`
	Struct              certificationPair      `json:"struct"`
	Array               []string               `json:"array"`
	Dictionary          map[string]string      `json:"dictionary"`
	Stringset           map[string]struct{}    `json:"stringset"`
	Nullable            *string                `json:"nullable"`
	NullableArrayStruct *[]certificationPair   `json:"nullable_array_struct"`
	Interface           certificationInterface `json:"interface"`
}

type certificationPair struct {
	First  int64  `json:"first"`
	Second string `json:"second"`
}

type certificationState struct {
	clientID string
}

func TestConnClientCertificationScenario(t *testing.T) {
	state := &certificationState{clientID: "client-1"}

	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		mustRegister := func(err error) {
			if err != nil {
				t.Fatalf("register certification handler: %v", err)
			}
		}

		mustRegister(b.RegisterUnary("org.varlink.certification", "Start", func(ctx context.Context, call UnaryCall) error {
			return call.Reply(ctx, struct {
				ClientID string `json:"client_id"`
			}{ClientID: state.clientID})
		}))

		mustRegister(b.RegisterUnary("org.varlink.certification", "Test01", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				ClientID string `json:"client_id"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			return call.Reply(ctx, struct {
				Bool bool `json:"bool"`
			}{Bool: true})
		}))

		mustRegister(b.RegisterUnary("org.varlink.certification", "Test02", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				ClientID string `json:"client_id"`
				Bool     bool   `json:"bool"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			if !in.Bool {
				return call.ReplyError(ctx, "org.varlink.certification.CertificationError", map[string]json.RawMessage{
					"wants": json.RawMessage(`true`),
					"got":   json.RawMessage(`false`),
				})
			}
			return call.Reply(ctx, struct {
				Int int64 `json:"int"`
			}{Int: 1})
		}))

		mustRegister(b.RegisterUnary("org.varlink.certification", "Test03", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				ClientID string `json:"client_id"`
				Int      int64  `json:"int"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			if in.Int != 1 {
				return call.ReplyError(ctx, "org.varlink.certification.CertificationError", nil)
			}
			return call.Reply(ctx, struct {
				Float float64 `json:"float"`
			}{Float: 1})
		}))

		mustRegister(b.RegisterUnary("org.varlink.certification", "Test04", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				ClientID string  `json:"client_id"`
				Float    float64 `json:"float"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			if in.Float != 1 {
				return call.ReplyError(ctx, "org.varlink.certification.CertificationError", nil)
			}
			return call.Reply(ctx, struct {
				String string `json:"string"`
			}{String: "ping"})
		}))

		mustRegister(b.RegisterUnary("org.varlink.certification", "Test05", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				ClientID string `json:"client_id"`
				String   string `json:"string"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			if in.String != "ping" {
				return call.ReplyError(ctx, "org.varlink.certification.CertificationError", nil)
			}
			return call.Reply(ctx, struct {
				Bool   bool    `json:"bool"`
				Int    int64   `json:"int"`
				Float  float64 `json:"float"`
				String string  `json:"string"`
			}{
				Bool:   false,
				Int:    2,
				Float:  3.141592653589793,
				String: "a lot of string",
			})
		}))

		mustRegister(b.RegisterUnary("org.varlink.certification", "Test06", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				ClientID string  `json:"client_id"`
				Bool     bool    `json:"bool"`
				Int      int64   `json:"int"`
				Float    float64 `json:"float"`
				String   string  `json:"string"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			if in.Bool || in.Int != 2 || in.Float != 3.141592653589793 || in.String != "a lot of string" {
				return call.ReplyError(ctx, "org.varlink.certification.CertificationError", nil)
			}
			return call.Reply(ctx, struct {
				Struct struct {
					Bool   bool    `json:"bool"`
					Int    int64   `json:"int"`
					Float  float64 `json:"float"`
					String string  `json:"string"`
				} `json:"struct"`
			}{
				Struct: struct {
					Bool   bool    `json:"bool"`
					Int    int64   `json:"int"`
					Float  float64 `json:"float"`
					String string  `json:"string"`
				}{
					Bool:   true,
					Int:    1,
					Float:  0.5,
					String: "next",
				},
			})
		}))

		mustRegister(b.RegisterUnary("org.varlink.certification", "Test07", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				ClientID string `json:"client_id"`
				Struct   struct {
					Bool   bool    `json:"bool"`
					Int    int64   `json:"int"`
					Float  float64 `json:"float"`
					String string  `json:"string"`
				} `json:"struct"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			if !in.Struct.Bool || in.Struct.Int != 1 || in.Struct.Float != 0.5 || in.Struct.String != "next" {
				return call.ReplyError(ctx, "org.varlink.certification.CertificationError", nil)
			}
			return call.Reply(ctx, struct {
				Map map[string]string `json:"map"`
			}{Map: map[string]string{"foo": "Foo", "bar": "Bar"}})
		}))

		mustRegister(b.RegisterUnary("org.varlink.certification", "Test08", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				ClientID string            `json:"client_id"`
				Map      map[string]string `json:"map"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			if !reflect.DeepEqual(in.Map, map[string]string{"foo": "Foo", "bar": "Bar"}) {
				return call.ReplyError(ctx, "org.varlink.certification.CertificationError", nil)
			}
			return call.Reply(ctx, struct {
				Set map[string]struct{} `json:"set"`
			}{Set: map[string]struct{}{"one": {}, "two": {}, "three": {}}})
		}))

		mustRegister(b.RegisterUnary("org.varlink.certification", "Test09", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				ClientID string              `json:"client_id"`
				Set      map[string]struct{} `json:"set"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			if len(in.Set) != 3 {
				return call.ReplyError(ctx, "org.varlink.certification.CertificationError", nil)
			}
			fooMap := map[string]string{"Foo": "foo", "Bar": "bar"}
			barMap := map[string]string{"one": "foo", "two": "bar"}
			foo := []*map[string]string{nil, &fooMap, nil, &barMap}
			return call.Reply(ctx, struct {
				MyType certificationMyType `json:"mytype"`
			}{
				MyType: certificationMyType{
					Object:     json.RawMessage(`{"method":"org.varlink.certification.Test09","parameters":{"map":{"foo":"Foo","bar":"Bar"}}}`),
					Enum:       "two",
					Struct:     certificationPair{First: 1, Second: "2"},
					Array:      []string{"one", "two", "three"},
					Dictionary: map[string]string{"foo": "Foo", "bar": "Bar"},
					Stringset:  map[string]struct{}{"one": {}, "two": {}, "three": {}},
					Nullable:   nil,
					Interface: certificationInterface{
						Foo: &foo,
						Anon: struct {
							Foo bool `json:"foo"`
							Bar bool `json:"bar"`
						}{
							Foo: true,
							Bar: false,
						},
					},
				},
			})
		}))

		mustRegister(b.RegisterStream("org.varlink.certification", "Test10", func(ctx context.Context, call StreamCall) error {
			var in struct {
				ClientID string              `json:"client_id"`
				MyType   certificationMyType `json:"mytype"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			if err := verifyCertificationMyType(in.MyType); err != nil {
				return call.ReplyError(ctx, "org.varlink.certification.CertificationError", map[string]string{"got": err.Error()})
			}
			for i := 1; i < 10; i++ {
				if err := call.Send(ctx, struct {
					String string `json:"string"`
				}{String: "Reply number " + strconv.Itoa(i)}); err != nil {
					return err
				}
			}
			return call.Close(ctx, struct {
				String string `json:"string"`
			}{String: "Reply number 10"})
		}))

		mustRegister(b.RegisterUnary("org.varlink.certification", "Test11", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				ClientID        string   `json:"client_id"`
				LastMoreReplies []string `json:"last_more_replies"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			want := []string{
				"Reply number 1",
				"Reply number 2",
				"Reply number 3",
				"Reply number 4",
				"Reply number 5",
				"Reply number 6",
				"Reply number 7",
				"Reply number 8",
				"Reply number 9",
				"Reply number 10",
			}
			if !call.IsOneway() || !reflect.DeepEqual(in.LastMoreReplies, want) {
				return call.ReplyError(ctx, "org.varlink.certification.CertificationError", nil)
			}
			return nil
		}))

		mustRegister(b.RegisterUnary("org.varlink.certification", "End", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				ClientID string `json:"client_id"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.ClientID != state.clientID {
				return call.ReplyError(ctx, "org.varlink.certification.ClientIdError", nil)
			}
			return call.Reply(ctx, struct {
				AllOK bool `json:"all_ok"`
			}{AllOK: true})
		}))
	})
	defer closeClientServer(t, client, done)

	ctx := context.Background()

	var start struct {
		ClientID string `json:"client_id"`
	}
	if err := client.Invoke(ctx, "org.varlink.certification.Start", nil, &start); err != nil {
		t.Fatalf("Start error = %v", err)
	}

	var test01 struct {
		Bool bool `json:"bool"`
	}
	if err := client.Invoke(ctx, "org.varlink.certification.Test01", struct {
		ClientID string `json:"client_id"`
	}{ClientID: start.ClientID}, &test01); err != nil {
		t.Fatalf("Test01 error = %v", err)
	}

	var test02 struct {
		Int int64 `json:"int"`
	}
	if err := client.Invoke(ctx, "org.varlink.certification.Test02", struct {
		ClientID string `json:"client_id"`
		Bool     bool   `json:"bool"`
	}{ClientID: start.ClientID, Bool: test01.Bool}, &test02); err != nil {
		t.Fatalf("Test02 error = %v", err)
	}

	var test03 struct {
		Float float64 `json:"float"`
	}
	if err := client.Invoke(ctx, "org.varlink.certification.Test03", struct {
		ClientID string `json:"client_id"`
		Int      int64  `json:"int"`
	}{ClientID: start.ClientID, Int: test02.Int}, &test03); err != nil {
		t.Fatalf("Test03 error = %v", err)
	}

	var test04 struct {
		String string `json:"string"`
	}
	if err := client.Invoke(ctx, "org.varlink.certification.Test04", struct {
		ClientID string  `json:"client_id"`
		Float    float64 `json:"float"`
	}{ClientID: start.ClientID, Float: test03.Float}, &test04); err != nil {
		t.Fatalf("Test04 error = %v", err)
	}

	var test05 struct {
		Bool   bool    `json:"bool"`
		Int    int64   `json:"int"`
		Float  float64 `json:"float"`
		String string  `json:"string"`
	}
	if err := client.Invoke(ctx, "org.varlink.certification.Test05", struct {
		ClientID string `json:"client_id"`
		String   string `json:"string"`
	}{ClientID: start.ClientID, String: test04.String}, &test05); err != nil {
		t.Fatalf("Test05 error = %v", err)
	}

	var test06 struct {
		Struct struct {
			Bool   bool    `json:"bool"`
			Int    int64   `json:"int"`
			Float  float64 `json:"float"`
			String string  `json:"string"`
		} `json:"struct"`
	}
	if err := client.Invoke(ctx, "org.varlink.certification.Test06", struct {
		ClientID string  `json:"client_id"`
		Bool     bool    `json:"bool"`
		Int      int64   `json:"int"`
		Float    float64 `json:"float"`
		String   string  `json:"string"`
	}{
		ClientID: start.ClientID,
		Bool:     test05.Bool,
		Int:      test05.Int,
		Float:    test05.Float,
		String:   test05.String,
	}, &test06); err != nil {
		t.Fatalf("Test06 error = %v", err)
	}

	var test07 struct {
		Map map[string]string `json:"map"`
	}
	if err := client.Invoke(ctx, "org.varlink.certification.Test07", struct {
		ClientID string `json:"client_id"`
		Struct   struct {
			Bool   bool    `json:"bool"`
			Int    int64   `json:"int"`
			Float  float64 `json:"float"`
			String string  `json:"string"`
		} `json:"struct"`
	}{ClientID: start.ClientID, Struct: test06.Struct}, &test07); err != nil {
		t.Fatalf("Test07 error = %v", err)
	}

	var test08 struct {
		Set map[string]struct{} `json:"set"`
	}
	if err := client.Invoke(ctx, "org.varlink.certification.Test08", struct {
		ClientID string            `json:"client_id"`
		Map      map[string]string `json:"map"`
	}{ClientID: start.ClientID, Map: test07.Map}, &test08); err != nil {
		t.Fatalf("Test08 error = %v", err)
	}

	var test09 struct {
		MyType certificationMyType `json:"mytype"`
	}
	if err := client.Invoke(ctx, "org.varlink.certification.Test09", struct {
		ClientID string              `json:"client_id"`
		Set      map[string]struct{} `json:"set"`
	}{ClientID: start.ClientID, Set: test08.Set}, &test09); err != nil {
		t.Fatalf("Test09 error = %v", err)
	}
	if err := verifyCertificationMyType(test09.MyType); err != nil {
		t.Fatalf("Test09 payload verification failed: %v", err)
	}

	stream, err := client.Stream(ctx, "org.varlink.certification.Test10", struct {
		ClientID string              `json:"client_id"`
		MyType   certificationMyType `json:"mytype"`
	}{ClientID: start.ClientID, MyType: test09.MyType})
	if err != nil {
		t.Fatalf("Test10 stream error = %v", err)
	}

	var replies []string
	for i := 1; i <= 10; i++ {
		var out struct {
			String string `json:"string"`
		}
		if err := stream.Recv(ctx, &out); err != nil {
			t.Fatalf("Test10 recv[%d] error = %v", i, err)
		}
		replies = append(replies, out.String)
	}
	if err := stream.Recv(ctx, nil); !errors.Is(err, io.EOF) {
		t.Fatalf("Test10 expected EOF after final reply, got %v", err)
	}
	wantReplies := []string{
		"Reply number 1",
		"Reply number 2",
		"Reply number 3",
		"Reply number 4",
		"Reply number 5",
		"Reply number 6",
		"Reply number 7",
		"Reply number 8",
		"Reply number 9",
		"Reply number 10",
	}
	if !reflect.DeepEqual(replies, wantReplies) {
		t.Fatalf("Test10 replies = %v, want %v", replies, wantReplies)
	}

	if err := client.Oneway(ctx, "org.varlink.certification.Test11", struct {
		ClientID        string   `json:"client_id"`
		LastMoreReplies []string `json:"last_more_replies"`
	}{ClientID: start.ClientID, LastMoreReplies: replies}); err != nil {
		t.Fatalf("Test11 error = %v", err)
	}

	var end struct {
		AllOK bool `json:"all_ok"`
	}
	if err := client.Invoke(ctx, "org.varlink.certification.End", struct {
		ClientID string `json:"client_id"`
	}{ClientID: start.ClientID}, &end); err != nil {
		t.Fatalf("End error = %v", err)
	}
	if !end.AllOK {
		t.Fatal("End returned all_ok=false")
	}
}

func verifyCertificationMyType(m certificationMyType) error {
	var object struct {
		Method     string `json:"method"`
		Parameters struct {
			Map map[string]string `json:"map"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(m.Object, &object); err != nil {
		return err
	}
	if object.Method != "org.varlink.certification.Test09" {
		return errors.New("wrong object method")
	}
	if !reflect.DeepEqual(object.Parameters.Map, map[string]string{"foo": "Foo", "bar": "Bar"}) {
		return errors.New("wrong object parameters")
	}
	if m.Enum != "two" {
		return errors.New("wrong enum")
	}
	if m.Struct != (certificationPair{First: 1, Second: "2"}) {
		return errors.New("wrong struct")
	}
	if !reflect.DeepEqual(m.Array, []string{"one", "two", "three"}) {
		return errors.New("wrong array")
	}
	if !reflect.DeepEqual(m.Dictionary, map[string]string{"foo": "Foo", "bar": "Bar"}) {
		return errors.New("wrong dictionary")
	}
	if len(m.Stringset) != 3 {
		return errors.New("wrong stringset length")
	}
	if _, ok := m.Stringset["one"]; !ok {
		return errors.New("stringset missing one")
	}
	if _, ok := m.Stringset["two"]; !ok {
		return errors.New("stringset missing two")
	}
	if _, ok := m.Stringset["three"]; !ok {
		return errors.New("stringset missing three")
	}
	if m.Nullable != nil {
		return errors.New("nullable must be nil")
	}
	if m.NullableArrayStruct != nil {
		return errors.New("nullable_array_struct must be nil")
	}
	if m.Interface.Foo == nil || len(*m.Interface.Foo) != 4 {
		return errors.New("wrong nested alias foo")
	}
	foo := *m.Interface.Foo
	if foo[0] != nil || foo[2] != nil {
		return errors.New("wrong nullable map entries")
	}
	if foo[1] == nil || !reflect.DeepEqual(*foo[1], map[string]string{"Foo": "foo", "Bar": "bar"}) {
		return errors.New("wrong foo[1]")
	}
	if foo[3] == nil || !reflect.DeepEqual(*foo[3], map[string]string{"one": "foo", "two": "bar"}) {
		return errors.New("wrong foo[3]")
	}
	if !m.Interface.Anon.Foo || m.Interface.Anon.Bar {
		return errors.New("wrong anon struct")
	}
	return nil
}
