package httputil

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	shareddomain "clinic-inventory/internal/domain/shared"

	"github.com/google/uuid"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

// WriteError はエラーをHTTPステータスにマッピングして返す。
// ErrNotFound→404, ErrConflict→409, それ以外はドメインのバリデーションエラーとみなし400。
func WriteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, shareddomain.ErrNotFound):
		WriteJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
	case errors.Is(err, shareddomain.ErrConflict):
		WriteJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
	case errors.Is(err, shareddomain.ErrForbidden):
		WriteJSON(w, http.StatusForbidden, errorResponse{Error: err.Error()})
	default:
		WriteJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	}
}

func DecodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// ParseID はパスパラメータ等のUUID文字列を共有ID型に変換する。
func ParseID(s string) (shareddomain.ID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return shareddomain.ID{}, errors.New("IDの形式が不正です: " + s)
	}
	return shareddomain.ID(u), nil
}

// CORS はローカル開発用のCORSミドルウェア。Remixの開発サーバーからのアクセスを許可する。
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
