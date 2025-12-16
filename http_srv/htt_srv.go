package http_srv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"goraft/test/node"
)

var _ http.Handler = (*HttpHandler)(nil)

type HttpHandler struct {
	rnode *node.RaftNode
}

func NewHttpHandler(rnode *node.RaftNode) *HttpHandler {
	return &HttpHandler{rnode: rnode}
}

func (h *HttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "only GET method supported", http.StatusMethodNotAllowed)
		return
	}

	var (
		err    error
		params map[string][]string = r.URL.Query()
		resp   map[string]string   = make(map[string]string)
		body   []byte
	)

	if strings.HasPrefix(r.URL.Path, "/get") {
		var (
			ok bool
			k  []string
			v  string
		)

		if k, ok = params["key"]; !ok {
			http.Error(w, "query parameter key is missting", http.StatusBadRequest)
			return
		}

		if v, ok = h.rnode.Get(k[0]); !ok {
			http.Error(w, "key not found", http.StatusNotFound)
			return
		}

		resp[k[0]] = v
	} else if strings.HasPrefix(r.URL.Path, "/put") {
		var (
			ok bool
			k  []string
			v  []string
		)

		if k, ok = params["key"]; !ok {
			http.Error(w, "query parameter key is missting", http.StatusBadRequest)
			return
		}
		if v, ok = params["val"]; !ok {
			http.Error(w, "query parameter val is missting", http.StatusBadRequest)
			return
		}

		if err = h.rnode.Set(k[0], v[0]); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp["put"] = fmt.Sprintf("%s=%v", k[0], v[0])
	} else if strings.HasPrefix(r.URL.Path, "/delete") {
		var (
			err error
			ok  bool
			k   []string
		)

		if k, ok = params["key"]; !ok {
			http.Error(w, "query parameter key is missting", http.StatusBadRequest)
			return
		}

		if err = h.rnode.Delete(k[0]); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp["deleted"] = k[0]
	} else if strings.HasPrefix(r.URL.Path, "/list") {
		var all map[string]string = h.rnode.GetAll()
		for k, v := range all {
			resp[k] = v
		}
	} else if strings.HasPrefix(r.URL.Path, "/join") {
		var (
			ok        bool
			id        []string
			advertise []string
		)

		if id, ok = params["id"]; !ok {
			http.Error(w, "query parameter id is missting", http.StatusBadRequest)
			return
		}
		if advertise, ok = params["advertise"]; !ok {
			http.Error(w, "query parameter advertise is missting", http.StatusBadRequest)
			return
		}
		if err = h.rnode.AddVoter(id[0], advertise[0]); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp["id"] = id[0]
		resp["advertise"] = advertise[0]
	} else if strings.HasPrefix(r.URL.Path, "/evict") {
		var (
			ok bool
			id []string
		)
		if id, ok = params["id"]; !ok {
			http.Error(w, "query parameter id is missting", http.StatusBadRequest)
			return
		}

		if err = h.rnode.DelVoter(id[0]); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp["evicted"] = id[0]
	} else if strings.HasPrefix(r.URL.Path, "/members") {
		members, err := h.rnode.ListMembers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for k, v := range members {
			resp[k] = v
		}
	} else {
		http.Error(w, "uri not impletmented", http.StatusNotImplemented)
		return
	}

	if body, err = json.Marshal(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err = w.Write(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
