package home

import (
	"encoding/json"
	"fmt"
	"github.com/keybase/client/go/gregor"
	"github.com/keybase/client/go/libkb"
	"github.com/keybase/client/go/protocol/gregor1"
	keybase1 "github.com/keybase/client/go/protocol/keybase1"
	"golang.org/x/net/context"
	"sync"
	"time"
)

type cache struct {
	obj      keybase1.HomeScreen
	cachedAt time.Time
}

type Home struct {
	libkb.Contextified

	sync.Mutex
	cache *cache
}

type rawGetHome struct {
	Status      libkb.AppStatus `json:"status"`
	VisitRecord *struct {
		Visits  int           `json:"visits"`
		Atime   keybase1.Time `json:"atime"`
		Version int           `json:"version"`
	} `json:"visit_record"`
	TodoList []struct {
		Badged bool                        `json:"badged"`
		Type   keybase1.HomeScreenTodoType `json:"type"`
	} `json:"todo_list"`
}

func (r *rawGetHome) GetAppStatus() *libkb.AppStatus {
	return &r.Status
}

func NewHome(g *libkb.GlobalContext) *Home {
	home := &Home{Contextified: libkb.NewContextified(g)}
	g.AddLogoutHook(home)
	return home
}

func (h *Home) get(ctx context.Context, markedViewed bool) (ret keybase1.HomeScreen, err error) {
	defer h.G().CTrace(ctx, "Home#get", func() error { return err })()

	arg := libkb.NewAPIArgWithNetContext(ctx, "home")
	arg.SessionType = libkb.APISessionTypeREQUIRED
	arg.Args = libkb.HTTPArgs{
		"record_visit": libkb.B{Val: markedViewed},
	}
	var raw rawGetHome
	if err = h.G().API.GetDecode(arg, &raw); err != nil {
		return ret, err
	}
	for _, item := range raw.TodoList {
		ret.Items = append(ret.Items, keybase1.HomeScreenItem{
			Data:   keybase1.NewHomeScreenItemDataWithTodo(keybase1.NewHomeScreenTodoDefault(item.Type)),
			Badged: item.Badged,
		})
	}
	if raw.VisitRecord != nil {
		ret.LastViewed = raw.VisitRecord.Atime
		ret.Version = raw.VisitRecord.Version
	}
	return ret, err
}

func (h *Home) Get(ctx context.Context, markViewed bool) (ret keybase1.HomeScreen, err error) {
	defer h.G().CTrace(ctx, "Home#Get", func() error { return err })()

	h.Lock()
	defer h.Unlock()

	if h.cache != nil {
		diff := h.G().GetClock().Now().Sub(h.cache.cachedAt)
		if diff < libkb.HomeCacheTimeout {
			h.G().Log.CDebugf(ctx, "| hit cache (cached %s ago)", diff)
			if markViewed {
				h.G().Log.CDebugf(ctx, "| going to server to mark view, anyways")
				tmpErr := h.markViewed(ctx)
				if tmpErr != nil {
					h.G().Log.CInfof(ctx, "Error marking home as viewed: %s", tmpErr.Error())
				}
			}
			return h.cache.obj, nil
		}
	}

	ret, err = h.get(ctx, markViewed)
	if err == nil {
		h.cache = &cache{
			obj:      ret,
			cachedAt: h.G().GetClock().Now(),
		}
	}
	return ret, err
}

func (h *Home) skipTodoType(ctx context.Context, typ keybase1.HomeScreenTodoType) (err error) {
	defer h.G().CTrace(ctx, "Home#SkipType", func() error { return err })()

	_, err = h.G().API.Post(libkb.APIArg{
		Endpoint:    "home/todo/skip",
		SessionType: libkb.APISessionTypeREQUIRED,
		Args: libkb.HTTPArgs{
			"type": libkb.I{Val: int(typ)},
		},
		NetContext: ctx,
	})

	return err
}

func (h *Home) bustCache(ctx context.Context) {
	h.G().Log.CDebugf(ctx, "Home#bustCache")
	h.Lock()
	defer h.Unlock()
	h.cache = nil
}

func (h *Home) SkipTodoType(ctx context.Context, typ keybase1.HomeScreenTodoType) (err error) {
	var which string
	var ok bool
	if which, ok = keybase1.HomeScreenTodoTypeRevMap[typ]; !ok {
		which = fmt.Sprintf("unknown=%d", int(typ))
	}
	defer h.G().CTrace(ctx, fmt.Sprintf("home#SkipTodoType(%s)", which), func() error { return err })()
	h.bustCache(ctx)
	return h.skipTodoType(ctx, typ)
}

func (h *Home) MarkViewed(ctx context.Context) (err error) {
	defer h.G().CTrace(ctx, "Home#MarkViewed", func() error { return err })()
	h.bustCache(ctx)
	return h.markViewed(ctx)
}

func (h *Home) markViewed(ctx context.Context) (err error) {
	defer h.G().CTrace(ctx, "Home#markViewed", func() error { return err })()

	_, err = h.G().API.Post(libkb.APIArg{
		Endpoint:    "home/visit",
		SessionType: libkb.APISessionTypeREQUIRED,
		Args:        libkb.HTTPArgs{},
		NetContext:  ctx,
	})
	return err
}

func (h *Home) ActionTaken(ctx context.Context) (err error) {
	defer h.G().CTrace(ctx, "Home#ActionTaken", func() error { return err })()
	h.bustCache(ctx)
	return err
}

func (h *Home) OnLogout() error {
	h.bustCache(context.Background())
	return nil
}

type updateGregorMessage struct {
	Version int `json:"version"`
}

func (h *Home) handleUpdate(ctx context.Context, item gregor.Item) (err error) {
	defer h.G().CTrace(ctx, "Home#handleUpdate", func() error { return err })()
	var msg updateGregorMessage
	if err = json.Unmarshal(item.Body().Bytes(), &msg); err != nil {
		h.G().Log.Debug("error unmarshaling home.update item: %s", err.Error())
		return err
	}
	h.G().Log.Debug("home.update unmarshaled: %+v", msg)

	h.Lock()
	defer h.Unlock()
	if h.cache != nil && msg.Version > h.cache.obj.Version {
		h.cache = nil
	}
	return nil
}

func (h *Home) IsAlive() bool {
	return true
}
func (h *Home) Name() string {
	return "Home"
}
func (h *Home) Create(ctx context.Context, cli gregor1.IncomingInterface, category string, ibm gregor.Item) (bool, error) {
	switch category {
	case "home.update":
		return true, h.handleUpdate(ctx, ibm)
	default:
		return false, fmt.Errorf("unknown home handler category: %q", category)
	}
}
func (h *Home) Dismiss(ctx context.Context, cli gregor1.IncomingInterface, category string, ibm gregor.Item) (bool, error) {
	return true, nil
}
