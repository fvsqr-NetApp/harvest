package conf

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

var testYml = "../../cmd/tools/doctor/testdata/testConfig.yml"

func TestGetPrometheusExporterPorts(t *testing.T) {
	loadTestData(testYml)
	// Test without checking
	ValidatePortInUse = true
	type args struct {
		pollerName string
	}

	type test struct {
		name    string
		args    args
		wantErr bool
	}
	tests := []test{
		{"success", args{pollerName: "unix-01"}, false},
		{"failure_no_free_port", args{pollerName: "cluster-02"}, true},
		{"failure_poller_name_does_not_exist", args{pollerName: "test1"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetPrometheusExporterPorts(tt.args.pollerName)
			fmt.Println(got)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPrometheusExporterPorts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && got == 0 {
				t.Errorf("GetPrometheusExporterPorts() got = %v, want %s", got, "non zero value")
			}
		})
	}
}

func TestGetPrometheusExporterPortsIssue284(t *testing.T) {
	loadTestData("../../cmd/tools/doctor/testdata/issue-284.yml")
	loadPrometheusExporterPortRangeMapping()
	got, _ := GetPrometheusExporterPorts("issue-284")
	if got != 0 {
		t.Fatalf("expected port to be 0 but was %d", got)
	}
}

func loadTestData(yml string) {
	err := loadHarvestConfig(yml)
	if err != nil {
		panic(err)
	}
}

func TestPollerStructDefaults(t *testing.T) {
	loadTestData(testYml)
	t.Run("poller exporters", func(t *testing.T) {
		poller, err := GetPoller2(testYml, "zeros")
		if err != nil {
			panic(err)
		}
		// the poller does not define exporters but defaults does
		if poller.Exporters == nil {
			t.Fatalf(`expected exporters to not be nil, but it was`)
		}
		if len(*poller.Exporters) != 1 {
			t.Fatalf(`expected 1 exporters but got %v`, *poller.Exporters)
		}
		expected := []string{"prometheusrange"}
		if !reflect.DeepEqual(*poller.Exporters, expected) {
			t.Fatalf(`expected collectors to be %v but was %v`, expected, *poller.Exporters)
		}
	})

	t.Run("poller collector", func(t *testing.T) {
		poller, err := GetPoller2(testYml, "cluster-01")
		if err != nil {
			panic(err)
		}
		// the poller does not define collectors but defaults does
		if poller.Collectors == nil {
			t.Fatalf(`expected collectors to not be nil, but it was`)
		}
		if len(*poller.Collectors) != 2 {
			t.Fatalf(`expected 2 collectors but got %v`, *poller.Collectors)
		}
		defaultT := []string{"default.yaml", "custom.yaml"}
		expected := []Collector{{Name: "Zapi", Templates: &defaultT}, {Name: "ZapiPerf", Templates: &defaultT}}
		if !reflect.DeepEqual(*poller.Collectors, expected) {
			t.Fatalf(`expected collectors to be %v but was %v`, expected, *poller.Collectors)
		}
	})

	t.Run("poller username", func(t *testing.T) {
		poller, err := GetPoller2(testYml, "zeros")
		if err != nil {
			panic(err)
		}
		// the poller does not define a username but defaults does
		if *poller.Username != "myuser" {
			t.Fatalf(`expected username to be [myuser] but was [%v]`, *poller.Username)
		}
	})
}

func TestPollerUnion(t *testing.T) {
	loadTestData(testYml)
	addr := "addr"
	user := "user"
	defaults := Poller{
		Addr:       &addr,
		Collectors: &[]Collector{{Name: "0"}, {Name: "1"}, {Name: "2"}, {Name: "3"}},
		Username:   &user,
	}
	var p Poller
	p.Union(&defaults)
	if *p.Username != "user" {
		t.Fatalf(`expected username to be [user] but was [%v]`, *p.Username)
	}
	if *p.Addr != "addr" {
		t.Fatalf(`expected addr to be [addr] but was [%v]`, *p.Addr)
	}
	if len(*p.Collectors) != 4 {
		t.Fatalf(`expected collectors to be have four elements but was [%v]`, *p.Collectors)
	}
	for i := 0; i < len(*p.Collectors); i++ {
		actual := (*p.Collectors)[i].Name
		if actual != strconv.Itoa(i) {
			t.Fatalf(`expected element at index=%d to be %d but was [%v]`, i, i, actual)
		}
	}

	name := "name"
	isKfs := true
	maxFiles := 314
	p2 := Poller{
		Username:    &name,
		Collectors:  &[]Collector{{Name: "10"}, {Name: "11"}, {Name: "12"}, {Name: "13"}},
		IsKfs:       &isKfs,
		LogMaxFiles: &maxFiles,
	}
	p2.Union(&defaults)
	if *p2.Username != "name" {
		t.Fatalf(`expected username to be [name] but was [%v]`, *p2.Username)
	}
	if *p2.IsKfs != true {
		t.Fatalf(`expected isKfs to be [true] but was [%v]`, *p2.IsKfs)
	}
	if *p2.LogMaxFiles != maxFiles {
		t.Fatalf(`expected LogMaxFiles to be [314] but was [%v]`, *p2.LogMaxFiles)
	}
	for i := 0; i < len(*p2.Collectors); i++ {
		actual := (*p2.Collectors)[i].Name
		if actual != strconv.Itoa(10+i) {
			t.Fatalf(`expected element at index=%d to be %d but was [%v]`, i, i+10, actual)
		}
	}
}

func TestFlowStyle(t *testing.T) {
	loadTestData(testYml)
	t.Run("poller with flow", func(t *testing.T) {
		poller, err := GetPoller2(testYml, "flow")
		if err != nil {
			panic(err)
		}
		if len(*poller.Collectors) != 1 {
			t.Fatalf(`expected there to be one collector but got %v`, len(*poller.Collectors))
		}
		if (*poller.Collectors)[0].Name != "Zapi" {
			t.Fatalf(`expected the first collector to be Zapi but got %v`, (*poller.Collectors)[0])
		}
		if len(*poller.Exporters) != 1 {
			t.Fatalf(`expected there to be one exporter but got %v`, len(*poller.Exporters))
		}
		if (*poller.Collectors)[0].Name != "Zapi" {
			t.Fatalf(`expected the first exporter to be prom but got %v`, (*poller.Exporters)[0])
		}
	})
}

func TestIssue271_PollerPanicsWhenExportDoesNotExist(t *testing.T) {
	path := "../../cmd/tools/doctor/testdata/testConfig.yml"
	node, _ := LoadConfig(path)
	poller := node.GetChildS("Pollers").GetChildS("issue-271")
	t.Run("Poller panics when exporter does not exist", func(t *testing.T) {
		exporters, err := GetUniqueExporters(poller, path)
		if err != nil {
			panic(err)
		}
		if exporters != nil {
			return
		}
	})
}

func TestQuotedPassword(t *testing.T) {
	loadTestData(testYml)
	t.Run("quoted password", func(t *testing.T) {
		poller, err := GetPoller2(testYml, "pass-with-escape")
		if err != nil {
			panic(err)
		}
		if poller.Password != "#pass" {
			t.Fatalf(`expected password to be #pass but got %v`, poller.Password)
		}
	})
}

func TestCollectorConfig(t *testing.T) {
	type test struct {
		name string
		path string
		want []string
	}
	tests := []test{
		{name: "normal", path: "testdata/normal.yaml", want: []string{"default.yaml"}},
		{name: "issue_396", path: "testdata/issue_396.yaml", want: []string{"limited1.yaml", "limited2.yaml", "limited3.yaml"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loadHarvestConfig(tt.path)
			if err != nil {
				panic(err)
			}
			poller, err := GetPoller2(tt.path, "DC-01")
			if err != nil {
				panic(err)
			}
			for i, tc := range tt.want {
				collectors := *poller.Collectors
				if tc != (*collectors[0].Templates)[i] {
					t.Errorf("want %s collector config, got %s", tt.want[i], tc)
				}
			}
		})
	}
}
