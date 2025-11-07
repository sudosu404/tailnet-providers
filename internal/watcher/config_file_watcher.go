package watcher

import (
	"sync"

	"github.com/sudosu404/providers/internal/common"
	"github.com/sudosu404/tailnet-utils/task"
)

var (
	configDirWatcher   *DirWatcher
	configDirWatcherMu sync.Mutex
)

// create a new file watcher for file under ConfigBasePath.
func NewConfigFileWatcher(filename string) Watcher {
	configDirWatcherMu.Lock()
	defer configDirWatcherMu.Unlock()

	if configDirWatcher == nil {
		t := task.RootTask("config_dir_watcher", false)
		configDirWatcher = NewDirectoryWatcher(t, common.ConfigBasePath)
	}
	return configDirWatcher.Add(filename)
}
