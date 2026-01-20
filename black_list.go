package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type BlacklistStore struct {
	sync.RWMutex
	IPs     map[string]bool `json:"ips"`
	Wallets map[string]bool `json:"wallets"`
}

var store = &BlacklistStore{
	IPs:     make(map[string]bool),
	Wallets: make(map[string]bool),
}

type UpdateRequest struct {
	Mode    string   `json:"mode"` // full / add / remove
	IPs     []string `json:"ips"`
	Wallets []string `json:"wallets"`
}

// 认证检查 - Easegress 调用
func authHandler(w http.ResponseWriter, r *http.Request) {
	ip := r.Header.Get("X-Real-Ip")
	var datamap map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&datamap); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	wallet, _ := datamap["wallet"].(string)

	// 日志打印认证请求信息
	fmt.Printf("authHandler request - ip: %s, wallet: %s\n", ip, wallet)

	store.RLock()
	blocked := store.IPs[ip] || (wallet != "" && store.Wallets[wallet])
	store.RUnlock()

	if blocked {
		w.WriteHeader(403)
		w.Write([]byte("{\"auth\":false}"))
		return
	}

	//{"auth":true}
	w.WriteHeader(200)
	w.Write([]byte("{\"auth\":true}"))
}

// 查看黑名单
func listHandler(w http.ResponseWriter, r *http.Request) {
	store.RLock()
	defer store.RUnlock()

	ips := make([]string, 0, len(store.IPs))
	for ip := range store.IPs {
		ips = append(ips, ip)
	}

	wallets := make([]string, 0, len(store.Wallets))
	for wallet := range store.Wallets {
		wallets = append(wallets, wallet)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ips":     ips,
		"wallets": wallets,
	})
}

// 更新黑名单
func updateHandler(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	store.Lock()
	defer store.Unlock()

	switch req.Mode {
	case "full":
		store.IPs = make(map[string]bool)
		store.Wallets = make(map[string]bool)
		for _, ip := range req.IPs {
			store.IPs[ip] = true
		}
		for _, wallet := range req.Wallets {
			store.Wallets[wallet] = true
		}
	case "add":
		for _, ip := range req.IPs {
			store.IPs[ip] = true
		}
		for _, wallet := range req.Wallets {
			store.Wallets[wallet] = true
		}
	case "remove":
		for _, ip := range req.IPs {
			delete(store.IPs, ip)
		}
		for _, wallet := range req.Wallets {
			delete(store.Wallets, wallet)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"ips_count":     len(store.IPs),
		"wallets_count": len(store.Wallets),
	})
}

// 从第三方初始化
func initHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	resp, err := http.Get(req.URL)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	var data struct {
		IPs     []string `json:"ips"`
		Wallets []string `json:"wallets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	store.Lock()
	store.IPs = make(map[string]bool)
	store.Wallets = make(map[string]bool)
	for _, ip := range data.IPs {
		store.IPs[ip] = true
	}
	for _, wallet := range data.Wallets {
		store.Wallets[wallet] = true
	}
	store.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"ips_count":     len(store.IPs),
		"wallets_count": len(store.Wallets),
	})
}

func main() {
	http.HandleFunc("/auth", authHandler)
	http.HandleFunc("/blacklist", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			listHandler(w, r)
		case "POST":
			updateHandler(w, r)
		default:
			http.Error(w, "Method not allowed", 405)
		}
	})
	http.HandleFunc("/blacklist/init", initHandler)

	println("Blacklist service running on :8888")
	http.ListenAndServe(":8888", nil)
}
