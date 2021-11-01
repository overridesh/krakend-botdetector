package mux

import (
	"net/http"

	botdetector "github.com/devopsfaith/krakend-botdetector"
	krakend "github.com/devopsfaith/krakend-botdetector/krakend"
	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
	"github.com/luraproject/lura/proxy"
	"github.com/luraproject/lura/router/mux"
	luramux "github.com/luraproject/lura/router/mux"
)

// New checks the configuration and, if required, wraps the handler factory with a bot detector middleware
func New(hf luramux.HandlerFactory, l logging.Logger) luramux.HandlerFactory {
	return func(cfg *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		next := hf(cfg, p)

		detectorCfg, err := krakend.ParseConfig(cfg.ExtraConfig)
		if err == krakend.ErrNoConfig {
			l.Debug("botdetector: ", err.Error())
			return next
		}

		if err != nil {
			l.Warning("botdetector: ", err.Error())
			return next
		}

		d, err := botdetector.New(detectorCfg)
		if err != nil {
			l.Warning("botdetector: unable to create the LRU detector:", err.Error())
			return next
		}

		return handler(d, next)
	}
}

func handler(f botdetector.DetectorFunc, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if f(r) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("bot rejected"))
			return
		}
		next(w, r)
	}
}

type middleware struct {
	detector botdetector.DetectorFunc
}

// Register checks the configuration and, if required, registers a bot detector middleware at the gin engine
func Register(cfg config.ExtraConfig, l logging.Logger) mux.HandlerMiddleware {
	detectorCfg, err := krakend.ParseConfig(cfg)
	if err == krakend.ErrNoConfig {
		l.Debug("botdetector middleware: ", err.Error())
		return nil
	}
	if err != nil {
		l.Warning("botdetector middleware: ", err.Error())
		return nil
	}
	d, err := botdetector.New(detectorCfg)
	if err != nil {
		l.Warning("botdetector middleware: unable to createt the LRU detector:", err.Error())
		return nil
	}

	return middleware{
		detector: d,
	}
}

func (m middleware) Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.detector(r) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("bot rejected"))
			return
		}
		h.ServeHTTP(w, r)
	})
}
