package deploy

import (
	"bytes"
	"context"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/build"
	"github.com/shipa-corp/ketch/internal/docker"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"os"
	"path"
	"testing"

	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/require"
)

type mockClient struct {
	getFn    func(counter int, obj runtime.Object) error
	createFn func(counter int, obj runtime.Object) error
	updateFn func(counter int, obj runtime.Object) error

	getCounter int
	createCounter int
	updateCounter int
}

func (m *mockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	if m.getFn != nil {
		m.getCounter++
		return m.getFn(m.getCounter, obj)
	}
	return nil
}

func (m *mockClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	if m.createFn != nil {
		m.createCounter++
		return m.createFn(m.createCounter, obj)
	}
	return nil
}

func (m *mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	if m.updateFn != nil {
		m.updateCounter++
		return m.updateFn(m.updateCounter, obj)
	}
	return nil
}

type dockerMocker struct {}
func (dockerMocker) Build(ctx context.Context, req docker.BuildRequest)(*docker.BuildResponse,error){
	return &docker.BuildResponse{
		Procfile: ".proc",
		ImageURI: "shipa/someimage:latest",
	}, nil
}
func(dockerMocker) Push(ctx context.Context, req docker.BuildRequest) error {
	return nil
}

type mockConfiger struct  {}

func (c mockConfiger) ConfigFile() (*registryv1.ConfigFile, error) {
	return &registryv1.ConfigFile{}, nil
}


func remoteImgFn(ref name.Reference, options ...remote.Option) (ImageConfiger, error) {
	return &mockConfiger{}, nil
}

func TestNewCommand(t *testing.T) {
	tt := []struct {
		name      string
		params    *Params
		arguments []string
		setup     func(t *testing.T)
		wantError bool
	}{
		{
			name: "happy path build from source",
			arguments: []string{
				"myapp",
				"src",
				"--platform", "go",
				"--pool", "mypool",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
			},
			params: &Params{
				Client:      &mockClient{
					getFn: func(counter int, obj runtime.Object) error {
						switch counter {
						case 1:
							return errors.NewNotFound(v1.Resource(""), "")
						case 2, 4:
							_, ok := obj.(*ketchv1.Pool)
							require.True(t, ok)
							return nil
						case 3:
							_, ok := obj.(*ketchv1.Platform)
							require.True(t, ok)
							return nil


						}

						panic("should not reach")
					},
				},
				KubeClient:  fake.NewSimpleClientset(),
				Builder:     build.GetSourceHandler(&dockerMocker{}),
				RemoteImage: remoteImgFn,
				Wait:        nil,
				Writer:      &bytes.Buffer{},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			cmd := NewCommand(tc.params)
			cmd.SetArgs(tc.arguments)
			err := cmd.Execute()
			if tc.wantError {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
		})
	}
}