package factory

import (
	"context"
	victoriametricsv1beta1 "github.com/VictoriaMetrics/operator/api/v1beta1"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sort"
	"testing"
)

func Test_selectNamespaces(t *testing.T) {
	type args struct {
		selector labels.Selector
	}
	tests := []struct {
		name         string
		args         args
		predefinedNs []*v1.Namespace
		want         []string
		wantErr      bool
	}{
		{
			name:         "select 1 ns",
			args:         args{selector: labels.SelectorFromValidatedSet(labels.Set{})},
			predefinedNs: []*v1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}}},
			want:         []string{"ns1"},
			wantErr:      false,
		},
		{
			name: "select 1 ns with label selector",
			args: args{selector: labels.SelectorFromValidatedSet(labels.Set{"name": "kube-system"})},
			predefinedNs: []*v1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "kube-system", Labels: map[string]string{"name": "kube-system"}}},
			},
			want:    []string{"kube-system"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []runtime.Object{}
			for _, n := range tt.predefinedNs {
				objs = append(objs, n)
			}
			s := scheme.Scheme
			client := fake.NewFakeClientWithScheme(s, objs...)
			got, err := selectNamespaces(context.TODO(), client, tt.args.selector)
			if (err != nil) != tt.wantErr {
				t.Errorf("selectNamespaces() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("selectNamespaces() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectRules(t *testing.T) {
	type args struct {
		p *victoriametricsv1beta1.VMAlert
		l logr.Logger
	}
	tests := []struct {
		name             string
		args             args
		predefinedObjets []runtime.Object
		want             []string
		wantErr          bool
	}{
		{
			name: "select default rule",
			args: args{
				p: &victoriametricsv1beta1.VMAlert{},
				l: logf.Log.WithName("unit-test"),
			},
			want: []string{"default-vmalert.yaml"},
		},
		{
			name: "select default rule additional rule from another namespace",
			args: args{
				p: &victoriametricsv1beta1.VMAlert{ObjectMeta: metav1.ObjectMeta{Name: "test-vm-alert", Namespace: "monitor"},
					Spec: victoriametricsv1beta1.VMAlertSpec{RuleNamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{}}, RuleSelector: &metav1.LabelSelector{}}},
				l: logf.Log.WithName("unit-test"),
			},
			predefinedObjets: []runtime.Object{
				//we need namespace for filter + object inside this namespace
				&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
				&victoriametricsv1beta1.VMRule{ObjectMeta: metav1.ObjectMeta{Name: "error-alert", Namespace: "default"}, Spec: victoriametricsv1beta1.VMRuleSpec{
					Groups: []victoriametricsv1beta1.RuleGroup{{Name: "error-alert", Interval: "10s", Rules: []victoriametricsv1beta1.Rule{
						{Alert: "", Expr: intstr.IntOrString{IntVal: 10}, For: "10s", Labels: nil, Annotations: nil},
					}}},
				}},
			},
			want: []string{"default-error-alert.yaml"},
		},
		{
			name: "select default rule, and additional rule from another namespace with namespace filter",
			args: args{
				p: &victoriametricsv1beta1.VMAlert{ObjectMeta: metav1.ObjectMeta{Name: "test-vm-alert", Namespace: "monitor"},
					Spec: victoriametricsv1beta1.VMAlertSpec{RuleNamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"monitoring": "enabled"}}, RuleSelector: &metav1.LabelSelector{}}},
				l: logf.Log.WithName("unit-test"),
			},
			predefinedObjets: []runtime.Object{
				//we need namespace for filter + object inside this namespace
				&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
				&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "monitoring", Labels: map[string]string{"monitoring": "enabled"}}},
				&victoriametricsv1beta1.VMRule{ObjectMeta: metav1.ObjectMeta{Name: "error-alert", Namespace: "default"}, Spec: victoriametricsv1beta1.VMRuleSpec{
					Groups: []victoriametricsv1beta1.RuleGroup{{Name: "error-alert", Interval: "10s", Rules: []victoriametricsv1beta1.Rule{
						{Alert: "", Expr: intstr.IntOrString{IntVal: 10}, For: "10s", Labels: nil, Annotations: nil},
					}}},
				}},
				&victoriametricsv1beta1.VMRule{ObjectMeta: metav1.ObjectMeta{Name: "error-alert-at-monitoring", Namespace: "monitoring"}, Spec: victoriametricsv1beta1.VMRuleSpec{
					Groups: []victoriametricsv1beta1.RuleGroup{{Name: "error-alert", Interval: "10s", Rules: []victoriametricsv1beta1.Rule{
						{Alert: "", Expr: intstr.IntOrString{IntVal: 10}, For: "10s", Labels: nil, Annotations: nil},
					}}},
				}},
			},
			want: []string{"monitoring-error-alert-at-monitoring.yaml"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := []runtime.Object{}
			obj = append(obj, tt.predefinedObjets...)
			s := scheme.Scheme
			s.AddKnownTypes(victoriametricsv1beta1.GroupVersion, &victoriametricsv1beta1.VMRule{}, &victoriametricsv1beta1.VMRuleList{})
			fclient := fake.NewFakeClientWithScheme(s, obj...)
			got, err := SelectRules(context.TODO(), tt.args.p, fclient)
			if (err != nil) != tt.wantErr {
				t.Errorf("SelectRules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotNames := []string{}
			for ruleName := range got {
				gotNames = append(gotNames, ruleName)
			}
			sort.Strings(gotNames)

			if !reflect.DeepEqual(gotNames, tt.want) {
				t.Errorf("SelectRules() got = %v, want %v", gotNames, tt.want)
			}
		})
	}
}

func TestCreateOrUpdateRuleConfigMaps(t *testing.T) {
	type args struct {
		cr *victoriametricsv1beta1.VMAlert
	}
	tests := []struct {
		name              string
		args              args
		want              []string
		wantErr           bool
		predefinedObjects []runtime.Object
	}{
		{
			name: "base-rules-gen",
			args: args{cr: &victoriametricsv1beta1.VMAlert{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "base-vmalert",
				},
			}},
			want: []string{"vm-base-vmalert-rulefiles-0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := []runtime.Object{}
			obj = append(obj, tt.predefinedObjects...)
			fclient := fake.NewFakeClientWithScheme(testGetScheme(), obj...)
			got, err := CreateOrUpdateRuleConfigMaps(context.TODO(), tt.args.cr, fclient)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateOrUpdateRuleConfigMaps() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateOrUpdateRuleConfigMaps() got = %v, want %v", got, tt.want)
			}
		})
	}
}
