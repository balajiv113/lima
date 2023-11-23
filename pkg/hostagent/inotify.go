package hostagent

import (
	"context"
	"github.com/lima-vm/lima/pkg/limagrpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"path"

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

	client, err := a.getOrCreateClient(ctx)
	if err != nil {
		logrus.Error("failed to create client for inotify", err)
	}
	inotifyCh := make(chan *limagrpc.InotifyResponse)
	err = client.Inotify(ctx, inotifyCh)
	if err != nil {
		logrus.Error("inotify call failed", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case watchEvent := <-mountWatchCh:
			stat, err := os.Stat(watchEvent.Path())
			if err != nil {
				continue
			}

			if filterEvents(watchEvent) {
				continue
			}

			event := &limagrpc.InotifyResponse{Location: watchEvent.Path(), Time: timestamppb.New(stat.ModTime().UTC())}
			inotifyCh <- event
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
