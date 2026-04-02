package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
)

func TestSyncWithIndex(t *testing.T) {
	testCases := []struct {
		Name                  string
		CurrentEntries        map[string]*UIPlugin
		ExpectedEntries       map[string]*UIPlugin
		FsCacheFiles          []string
		ShouldDelete          bool
		ExpectedPluginDeleted string
	}{
		{
			Name: "Sync index with FS cache no new entries",
			CurrentEntries: map[string]*UIPlugin{
				"test-plugin": {
					UIPluginEntry: v1.UIPluginEntry{
						Name:     "test-plugin",
						Version:  "0.1.0",
						Endpoint: "https://test.endpoint.svc",
						NoCache:  false,
						Metadata: map[string]string{
							"test": "data",
						},
					},
				},
			},
			ExpectedEntries: map[string]*UIPlugin{
				"test-plugin": {
					UIPluginEntry: v1.UIPluginEntry{
						Name:     "test-plugin",
						Version:  "0.1.0",
						Endpoint: "https://test.endpoint.svc",
						NoCache:  false,
						Metadata: map[string]string{
							"test": "data",
						},
					},
				},
			},
			FsCacheFiles: []string{
				FSCacheRootDir + "/test-plugin/0.1.0",
			},
		},
		{
			Name: "Sync index with FS cache delete old test-plugin-2 entry",
			CurrentEntries: map[string]*UIPlugin{
				"test-plugin": {
					UIPluginEntry: v1.UIPluginEntry{
						Name:     "test-plugin",
						Version:  "0.1.0",
						Endpoint: "https://test.endpoint.svc",
						NoCache:  false,
						Metadata: map[string]string{
							"test": "data",
						},
					},
				},
			},
			ExpectedEntries: map[string]*UIPlugin{
				"test-plugin": {
					UIPluginEntry: v1.UIPluginEntry{
						Name:     "test-plugin",
						Version:  "0.1.0",
						Endpoint: "https://test.endpoint.svc",
						NoCache:  false,
						Metadata: map[string]string{
							"test": "data",
						},
					},
				},
			},
			FsCacheFiles: []string{
				FSCacheRootDir + "/test-plugin/0.1.0",
				FSCacheRootDir + "/test-plugin-2/0.1.0",
			},
			ShouldDelete:          true,
			ExpectedPluginDeleted: FSCacheRootDir + "/test-plugin-2",
		},
		{
			Name: "Sync with nil entry does not panic",
			CurrentEntries: map[string]*UIPlugin{
				"test-plugin": nil,
			},
			ExpectedEntries: map[string]*UIPlugin{
				"test-plugin": nil,
			},
			FsCacheFiles: []string{
				FSCacheRootDir + "/test-plugin/0.1.0",
			},
			ShouldDelete:          true,
			ExpectedPluginDeleted: FSCacheRootDir + "/test-plugin",
		},
		{
			Name:            "Sync with nil UIPluginEntry does not panic",
			CurrentEntries:  map[string]*UIPlugin{},
			ExpectedEntries: map[string]*UIPlugin{},
			FsCacheFiles: []string{
				FSCacheRootDir + "/test-plugin/0.1.0",
			},
			ShouldDelete:          true,
			ExpectedPluginDeleted: FSCacheRootDir + "/test-plugin",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			var (
				osRemoveAllCalled   bool
				actualPluginDeleted string
			)
			if tc.ShouldDelete {
				osRemoveAll = func(path string) error {
					osRemoveAllCalled = true
					actualPluginDeleted = path
					return nil
				}
			}
			fsCache := FSCache{}
			index := SafeIndex{
				Entries: tc.CurrentEntries,
			}
			err := fsCache.SyncWithIndex(&index, tc.FsCacheFiles)
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, index.Entries, tc.CurrentEntries)
			if tc.ShouldDelete {
				assert.Equal(t, tc.ShouldDelete, osRemoveAllCalled)
				if tc.ExpectedPluginDeleted != "" {
					assert.Equal(t, tc.ExpectedPluginDeleted, actualPluginDeleted)
				}
			}
		})
	}
}

func Test_isCached(t *testing.T) {
	testCases := []struct {
		Name          string
		PluginName    string
		PluginVersion string
		Expected      bool
		OsStatErr     error
		IsDirEmptyVal bool
	}{
		{
			Name:          "Test cached plugin",
			PluginName:    "test-plugin",
			PluginVersion: "0.0.1",
			Expected:      true,
			OsStatErr:     nil,
			IsDirEmptyVal: false,
		},
		{
			Name:          "Test non cached plugin",
			PluginName:    "test-plugin",
			PluginVersion: "0.0.1",
			Expected:      false,
			OsStatErr:     os.ErrNotExist,
			IsDirEmptyVal: true,
		},
		{
			Name:          "Test cached plugin but dir is empty",
			PluginName:    "test-plugin",
			PluginVersion: "0.0.1",
			Expected:      false,
			OsStatErr:     nil,
			IsDirEmptyVal: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			osStat = func(name string) (fs.FileInfo, error) {
				return nil, tc.OsStatErr
			}
			isDirEmpty = func(path string) (bool, error) {
				return tc.IsDirEmptyVal, nil
			}
			fsCache := FSCache{}
			actual, err := fsCache.isCached(tc.PluginName, tc.PluginVersion)
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, tc.Expected, actual)
		})
	}
}

func Test_getChartNameAndVersion(t *testing.T) {
	testCases := []struct {
		name                 string
		input                string
		expectedChartName    string
		expectedChartVersion string
		expectedError        error
	}{
		{
			name:                 "chart rooted at FSCacheRootDir",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "istio/v2.1.0"),
			expectedChartName:    "istio",
			expectedChartVersion: "v2.1.0",
			expectedError:        nil,
		},
		{
			name:                 "chart rooted at FSCacheRootDir with rc version",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "istio/v2.1.0-rc1"),
			expectedChartName:    "istio",
			expectedChartVersion: "v2.1.0-rc1",
			expectedError:        nil,
		},
		{
			name:                 "file rooted at FSCacheRootDir",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "istio/2.1.0/Chart.yaml"),
			expectedChartName:    "istio",
			expectedChartVersion: "2.1.0",
			expectedError:        nil,
		},
		{
			name:                 "chart rooted at FSCacheRootDir without version",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "istio"),
			expectedChartName:    "",
			expectedChartVersion: "",
			expectedError:        fmt.Errorf("file path is not valid"),
		},
		{
			name:                 "chart rooted at FSCacheRootDir with invalid version",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "istio/invalid-version"),
			expectedChartName:    "",
			expectedChartVersion: "",
			expectedError:        fmt.Errorf("invalid chart version"),
		},
		{
			name:                 "chart not rooted at FSCacheRootDir",
			input:                "/home/wrong-path/bad-chart/v1.0.0",
			expectedChartName:    "",
			expectedChartVersion: "",
			expectedError:        fmt.Errorf("path root is not the root cache path"),
		},
		{
			name:                 "chart not rooted at FSCacheRootDir with ../",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "../bad-chart/v2.1.0"),
			expectedChartName:    "",
			expectedChartVersion: "",
			expectedError:        fmt.Errorf("path root is not the root cache path"),
		},
		{
			name:                 "chart not rooted at FSCacheRootDir 2",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "/istio/../../bad-chart/v2.1.0"),
			expectedChartName:    "",
			expectedChartVersion: "",
			expectedError:        fmt.Errorf("path root is not the root cache path"),
		},
	}

	for _, testCase := range testCases {

		chartName, chartVersion, err := getChartNameAndVersion(testCase.input)
		assert.Equal(t, testCase.expectedChartName, chartName, testCase.name)
		assert.Equal(t, testCase.expectedChartVersion, chartVersion, testCase.name)
		if testCase.expectedError != nil {
			assert.ErrorContains(t, err, testCase.expectedError.Error(), testCase.name)
		} else {
			assert.NoError(t, err, testCase.name)
		}
	}
}

func Test_validateFilesTxtEntries(t *testing.T) {
	testCases := []struct {
		Name      string
		Files     []string
		ShouldErr bool
	}{
		{
			Name:      "valid files",
			Files:     []string{"plugin/file.js", "plugin/folder/file.min.js"},
			ShouldErr: false,
		},
		{
			Name:      "valid encoded files",
			Files:     []string{"plugins%2Ffile.js", "plugin%2Ffolder%2Ffile.min.js"},
			ShouldErr: false,
		},
		{
			Name:      "invalid file starting with /",
			Files:     []string{"/plugin/file.js", "plugin/folder/file.js"},
			ShouldErr: true,
		},
		{
			Name:      "invalid encoded file starting with /",
			Files:     []string{"%2Fplugin%2Ffile.js", "plugin/folder/file.js"},
			ShouldErr: true,
		},
		{
			Name:      "invalid file starting with \\",
			Files:     []string{"\\plugin/file.js", "plugin/folder/file.js"},
			ShouldErr: true,
		},
		{
			Name:      "invalid file starting with ..",
			Files:     []string{"plugin/file.js", "../plugin/folder/file.js"},
			ShouldErr: true,
		},
		{
			Name:      "invalid file starting with .",
			Files:     []string{"plugin/file.js", ".plugin/folder/file.js"},
			ShouldErr: true,
		},
		{
			Name:      "invalid file containing ..",
			Files:     []string{"plugin/file.js", "plugin/folder/../file.js"},
			ShouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			err := validateFilesTxtEntries(tc.Files)
			if tc.ShouldErr {
				assert.ErrorContains(t, err, "invalid file entry")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func makeTarGz(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	data := []byte(content)
	assert.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}))
	_, err := tw.Write(data)
	assert.NoError(t, err)
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestUntar_PathTraversal(t *testing.T) {
	testCases := []struct {
		Name      string
		TarEntry  string
		ShouldErr bool
	}{
		{Name: "relative traversal", TarEntry: "../../etc/passwd", ShouldErr: true},
		{Name: "deep traversal", TarEntry: "../../../tmp/pwned", ShouldErr: true},
		{Name: "single parent", TarEntry: "../sibling", ShouldErr: true},
		{Name: "absolute path", TarEntry: "/tmp/absolute", ShouldErr: true},
		{Name: "valid nested path", TarEntry: "plugin/index.js", ShouldErr: false},
		{Name: "valid flat file", TarEntry: "readme.txt", ShouldErr: false},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "cache")
			assert.NoError(t, os.MkdirAll(dst, 0755))

			tarGz := makeTarGz(t, tc.TarEntry, "test")
			err := Untar(dst, bytes.NewReader(tarGz))
			if tc.ShouldErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid path")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEndpointURL(t *testing.T) {
	testCases := []struct {
		Name      string
		URL       string
		ShouldErr bool
	}{
		{Name: "loopback ipv4", URL: "http://127.0.0.1/a.tar.gz", ShouldErr: true},
		{Name: "localhost", URL: "http://localhost/a.tar.gz", ShouldErr: true},
		{Name: "private 10.x", URL: "http://10.0.0.1/a.tar.gz", ShouldErr: true},
		{Name: "private 172.16.x", URL: "http://172.16.0.1/a.tar.gz", ShouldErr: true},
		{Name: "private 192.168.x", URL: "http://192.168.1.1/a.tar.gz", ShouldErr: true},
		{Name: "link-local metadata", URL: "http://169.254.169.254/latest/", ShouldErr: true},
		{Name: "ftp scheme", URL: "ftp://evil.com/a.tar.gz", ShouldErr: true},
		{Name: "file scheme", URL: "file:///etc/passwd", ShouldErr: true},
		{Name: "empty host", URL: "http:///path", ShouldErr: true},
		{Name: "ipv6 loopback", URL: "http://[::1]/a.tar.gz", ShouldErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			err := validateEndpointURL(tc.URL)
			if tc.ShouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
