package sam

import "testing"

func addrEq(a1 *Address, a2 *Address) bool {
	eq := a1.Type == a2.Type && a1.Arg == a2.Arg
	if a1.End != nil && a2.End != nil {
		return eq && addrEq(a1.End, a2.End)
	}
	return eq
}

func testParseAddress(t *testing.T) {
	tests := []struct {
		src string
		res Address
	}{
		// Valid addresses.
		{",", Address{Type: '0', Arg: "", End: &Address{Type: '$', Arg: "", End: nil}}},
		{",$", Address{Type: '0', Arg: "", End: &Address{Type: '$', Arg: "", End: nil}}},
		{"0,", Address{Type: '0', Arg: "", End: &Address{Type: '$', Arg: "", End: nil}}},
		{"0,$", Address{Type: '0', Arg: "", End: &Address{Type: '$', Arg: "", End: nil}}},
		{"1,", Address{Type: 'l', Arg: "1", End: &Address{Type: '$', Arg: "", End: nil}}},
		{",2", Address{Type: '0', Arg: "", End: &Address{Type: 'l', Arg: "2", End: nil}}},
		{"3,4", Address{Type: 'l', Arg: "3", End: &Address{Type: 'l', Arg: "4", End: nil}}},
		{"#,#", Address{Type: '#', Arg: "", End: &Address{Type: '#', Arg: "", End: nil}}},
		{"#5,#6", Address{Type: '#', Arg: "5", End: &Address{Type: '#', Arg: "6", End: nil}}},
		{"#77,#88", Address{Type: '#', Arg: "77", End: &Address{Type: '#', Arg: "88", End: nil}}},
		{"#9,/a/", Address{Type: '#', Arg: "9", End: &Address{Type: '/', Arg: "a", End: nil}}},
		{"/b/,/c/", Address{Type: '/', Arg: "b", End: &Address{Type: '/', Arg: "c", End: nil}}},
		{"//", Address{Type: '/', Arg: "", End: nil}},
		{"/dddd/", Address{Type: '/', Arg: "dddd", End: nil}},
		{"0", Address{Type: '0', Arg: "", End: nil}},
		{"$", Address{Type: '$', Arg: "", End: nil}},
		{"10", Address{Type: 'l', Arg: "10", End: nil}},
		{"#", Address{Type: '#', Arg: "", End: nil}},
		{"#11", Address{Type: '#', Arg: "11", End: nil}},
	}
	var p Parser
	for _, test := range tests {
		p.Init([]byte(test.src))
		addr, _, _ := p.Parse()
		if !addrEq(addr, &test.res) {
			t.Errorf("got:%q, want:%q", addr, test.res)
		}
	}
	p.Init([]byte(",,"))
	_, _, err := p.Parse()
	if err == nil {
		t.Errorf(`expected parser error when parsing ",,"`)
	}
}

func cmdEq(c1 *Command, c2 *Command) bool {
	eq := c1.Name == c2.Name && c1.Arg == c2.Arg
	if c1.Next == nil && c2.Next != nil || c1.Next != nil && c2.Next == nil {
		return false
	}
	if c1.Next != nil && c2.Next != nil {
		return eq && cmdEq(c1.Next, c2.Next)
	}
	return eq
}

func cmdListEq(l1 []*Command, l2 []*Command) bool {
	if len(l1) != len(l2) {
		return false
	}
	for i, cmd1 := range l1 {
		if !cmdEq(cmd1, l2[i]) {
			return false
		}
	}
	return true
}

func testParseCommand(t *testing.T) {
	tests := []struct {
		src string
		res []*Command
	}{
		// Valid commands.
		{"a/aaa/", []*Command{
			&Command{Name: "a", Arg: "aaa"},
		}},
		{"i/iii/", []*Command{
			&Command{Name: "i", Arg: "iii"},
		}},
		{"c/ccc/", []*Command{
			&Command{Name: "c", Arg: "ccc"},
		}},
		{"c/111/a/222/i/333", []*Command{
			&Command{Name: "c", Arg: "111"},
			&Command{Name: "a", Arg: "222"},
			&Command{Name: "i", Arg: "333"},
		}},
		{"x/xxx/", []*Command{
			&Command{Name: "x", Arg: "xxx"},
		}},
		{"g/ggg/", []*Command{
			&Command{Name: "g", Arg: "ggg"},
		}},
		{"v/vvv/", []*Command{
			&Command{Name: "v", Arg: "vvv"},
		}},
		{"x/xxx/a/foo", []*Command{
			&Command{Name: "x", Arg: "xxx", Next: &Command{Name: "a", Arg: "foo"}},
		}},
		{"i/foo/x/xxx/a/bar", []*Command{
			&Command{Name: "i", Arg: "foo"},
			&Command{Name: "x", Arg: "xxx", Next: &Command{Name: "a", Arg: "bar"}},
		}},
		{"c/coo/x/xoo/g/goo/i/foo", []*Command{
			&Command{Name: "c", Arg: "coo"},
			&Command{
				Name: "x", Arg: "xoo",
				Next: &Command{
					Name: "g", Arg: "goo", Next: &Command{Name: "i", Arg: "foo"},
				},
			},
		}},
		{"i/iii/c/coo/x/xoo/g/goo/g/loo/i/foo/i/bar/", []*Command{
			&Command{Name: "i", Arg: "iii"},
			&Command{Name: "c", Arg: "coo"},
			&Command{
				Name: "x", Arg: "xoo",
				Next: &Command{
					Name: "g", Arg: "goo",
					Next: &Command{
						Name: "g", Arg: "loo",
						Next: &Command{Name: "i", Arg: "foo"},
					},
				},
			},
			&Command{Name: "i", Arg: "bar"},
		}},
	}
	var p Parser
	for _, test := range tests {
		p.Init([]byte(test.src))
		_, cmdList, _ := p.Parse()
		if !cmdListEq(cmdList, test.res) {
			t.Errorf("got:%q, want:%q", cmdList, test.res)
		}
	}
}

func testParseCompound(t *testing.T) {
	tests := []struct {
		src     string
		addr    *Address
		cmdList []*Command
	}{
		{"20,29x/xxx/a/foo",
			&Address{Type: 'l', Arg: "20", End: &Address{Type: 'l', Arg: "29"}},
			[]*Command{
				&Command{Name: "x", Arg: "xxx", Next: &Command{Name: "a", Arg: "foo"}},
			}},
	}
	var p Parser
	for _, test := range tests {
		p.Init([]byte(test.src))
		addr, cmdList, _ := p.Parse()
		if !addrEq(addr, test.addr) {
			t.Errorf("addr: got:%q, want:%q", addr, test.addr)
		}
		if !cmdListEq(cmdList, test.cmdList) {
			t.Errorf("cmdList: got:%q, want:%q", cmdList, test.cmdList)
		}
	}
}

// TODO: Test for invalid # addresses and invalid commands.
func TestParser(t *testing.T) {
	testParseAddress(t)
	testParseCommand(t)
	testParseCompound(t)

}
