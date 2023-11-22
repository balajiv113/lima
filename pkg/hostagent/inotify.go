package hostagent

import (
	"context"
	"os"
	"path"

	guestagentapi "github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/rjeczalik/notify"
	"github.com/sirupsen/logrus"
)

const CacheSize = 10000

var inotifyCache = make(map[string]string)

func (a *HostAgent) startInotify(ctx context.Context) error {
	mountWatchCh := make(chan notify.EventInfo, 128)
	err := a.setupWatchers(mountWatchCh)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case watchEvent := <-mountWatchCh:
			client, err := a.getOrCreateClient(ctx)
			if err != nil {
				logrus.Error("failed to create client for inotify", err)
			}
			stat, err := os.Stat(watchEvent.Path())
			if err != nil {
				continue
			}

			if filterEvents(watchEvent) {
				continue
			}

			event := guestagentapi.InotifyEvent{Location: watchEvent.Path(), Time: stat.ModTime().UTC()}
			err = client.Inotify(ctx, event)

			if err != nil {
				logrus.WithError(err).Warn("failed to send inotify", err)
			}
		}
	}
}

func (a *HostAgent) setupWatchers(events chan notify.EventInfo) error {
	for _, m := range a.y.Mounts {
		if *m.Writable {
			location, err := localpathutil.Expand(m.Location)
			if err != nil {
				return err
			}
			logrus.Infof("enable inotify for writable mount: %s", location)
			err = notify.Watch(path.Join(location, "..."), events, notify.Create|notify.Write)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func filterEvents(event notify.EventInfo) bool {
	eventPath := event.Path()
	_, ok := inotifyCache[eventPath]
	if ok {
		// Ignore the duplicate inotify on mounted directories, so always remove a entry if already present
		delete(inotifyCache, eventPath)
		return true
	}
	inotifyCache[eventPath] = ""

	if len(inotifyCache) >= CacheSize {
		inotifyCache = make(map[string]string)
	}
	return false
}
