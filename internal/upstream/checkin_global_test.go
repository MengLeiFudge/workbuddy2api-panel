package upstream

import (
	"net/http"
	"testing"

	"github.com/misakano7545/workbuddy2api-panel/internal/auth"
)

// 活动积分配置不能当作签到回执，状态读取失败也不能触发写入。
func TestGlobalCheckinReceiptActivityGate(t *testing.T) {
	previous := auth.GlobalEnabled()
	auth.SetGlobalEnabled(true)
	t.Cleanup(func() { auth.SetGlobalEnabled(previous) })
	for _, tc := range []struct {
		name     string
		activity string
		status   int
		claim    string
		want     CheckinState
		writes   int
		credit   int
		known    bool
		wantErr  bool
	}{
		{"inactive", `{"active":false,"daily_credit":100,"today_credit":100}`, 200, "", CheckinInactive, 0, 0, false, true},
		{"already", `{"active":true,"today_checked_in":true,"today_credit":100,"streak_days":3}`, 200, "", CheckinAlready, 0, 0, false, false},
		{"fresh", `{"active":true,"today_checked_in":false,"daily_credit":100}`, 200, `{"credit":17,"streak_days":4}`, CheckinChecked, 1, 17, true, false},
		{"fresh unknown credit", `{"active":true}`, 200, `{"code":0,"data":{}}`, CheckinChecked, 1, 0, false, false},
		{"missing active", `{}`, 200, "", CheckinFailed, 0, 0, false, true},
		{"null state", `null`, 200, "", CheckinFailed, 0, 0, false, true},
		{"invalid active", `{"active":"true"}`, 200, "", CheckinFailed, 0, 0, false, true},
		{"unauthorized state", `{}`, 401, "", CheckinFailed, 0, 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var reads, writes int
			c := testClient(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case checkinActivityPath:
					reads++
					return jsonResp(tc.status, `{"code":0,"data":`+tc.activity+`}`), nil
				case dailyCheckinPath:
					writes++
					return jsonResp(200, tc.claim), nil
				default:
					t.Fatalf("unexpected global check-in request: %s", r.URL.Path)
					return nil, nil
				}
			})
			c.GlobalEnabled = true
			receipt, err := c.CheckinReceipt(globalAuth())
			if reads != 1 || writes != tc.writes {
				t.Fatalf("reads/writes=%d/%d, want 1/%d", reads, writes, tc.writes)
			}
			if receipt.State != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("receipt=%+v err=%v, want state=%s error=%v", receipt, err, tc.want, tc.wantErr)
			}
			if receipt.CreditKnown != tc.known || receipt.Credit != tc.credit {
				t.Fatalf("credit=%d known=%v, want %d/%v", receipt.Credit, receipt.CreditKnown, tc.credit, tc.known)
			}
		})
	}
}
