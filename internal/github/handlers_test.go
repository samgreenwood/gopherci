	"bytes"
	"context"
	"sync"
	"github.com/bradleyfalzon/gopherci/internal/queue"
// test integration key
var integrationKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEA0BUezcR7uycgZsfVLlAf4jXP7uFpVh4geSTY39RvYrAll0yh
q7uiQypP2hjQJ1eQXZvkAZx0v9lBYJmX7e0HiJckBr8+/O2kARL+GTCJDJZECpjy
97yylbzGBNl3s76fZ4CJ+4f11fCh7GJ3BJkMf9NFhe8g1TYS0BtSd/sauUQEuG/A
3fOJxKTNmICZr76xavOQ8agA4yW9V5hKcrbHzkfecg/sQsPMmrXixPNxMsqyOMmg
jdJ1aKr7ckEhd48ft4bPMO4DtVL/XFdK2wJZZ0gXJxWiT1Ny41LVql97Odm+OQyx
tcayMkGtMb1nwTcVVl+RG2U5E1lzOYpcQpyYFQIDAQABAoIBAAfUY55WgFlgdYWo
i0r81NZMNBDHBpGo/IvSaR6y/aX2/tMcnRC7NLXWR77rJBn234XGMeQloPb/E8iw
vtjDDH+FQGPImnQl9P/dWRZVjzKcDN9hNfNAdG/R9JmGHUz0JUddvNNsIEH2lgEx
C01u/Ntqdbk+cDvVlwuhm47MMgs6hJmZtS1KDPgYJu4IaB9oaZFN+pUyy8a1w0j9
RAhHpZrsulT5ThgCra4kKGDNnk2yfI91N9lkP5cnhgUmdZESDgrAJURLS8PgInM4
YPV9L68tJCO4g6k+hFiui4h/4cNXYkXnaZSBUoz28ICA6e7I3eJ6Y1ko4ou+Xf0V
csM8VFkCgYEA7y21JfECCfEsTHwwDg0fq2nld4o6FkIWAVQoIh6I6o6tYREmuZ/1
s81FPz/lvQpAvQUXGZlOPB9eW6bZZFytcuKYVNE/EVkuGQtpRXRT630CQiqvUYDZ
4FpqdBQUISt8KWpIofndrPSx6JzI80NSygShQsScWFw2wBIQAnV3TpsCgYEA3reL
L7AwlxCacsPvkazyYwyFfponblBX/OvrYUPPaEwGvSZmE5A/E4bdYTAixDdn4XvE
ChwpmRAWT/9C6jVJ/o1IK25dwnwg68gFDHlaOE+B5/9yNuDvVmg34PWngmpucFb/
6R/kIrF38lEfY0pRb05koW93uj1fj7Uiv+GWRw8CgYEAn1d3IIDQl+kJVydBKItL
tvoEur/m9N8wI9B6MEjhdEp7bXhssSvFF/VAFeQu3OMQwBy9B/vfaCSJy0t79uXb
U/dr/s2sU5VzJZI5nuDh67fLomMni4fpHxN9ajnaM0LyI/E/1FFPgqM+Rzb0lUQb
yqSM/ptXgXJls04VRl4VjtMCgYEAprO/bLx2QjxdPpXGFcXbz6OpsC92YC2nDlsP
3cfB0RFG4gGB2hbX/6eswHglLbVC/hWDkQWvZTATY2FvFps4fV4GrOt5Jn9+rL0U
elfC3e81Dw+2z7jhrE1ptepprUY4z8Fu33HNcuJfI3LxCYKxHZ0R2Xvzo+UYSBqO
ng0eTKUCgYEAxW9G4FjXQH0bjajntjoVQGLRVGWnteoOaQr/cy6oVii954yNMKSP
rezRkSNbJ8cqt9XQS+NNJ6Xwzl3EbuAt6r8f8VO1TIdRgFOgiUXRVNZ3ZyW8Hegd
kGTL0A6/0yAu9qQZlFbaD5bWhQo7eyx63u4hZGppBhkTSPikOYUPCH8=
-----END RSA PRIVATE KEY-----`)

type mockAnalyser struct {
	executed int
}

func (a *mockAnalyser) NewExecuter() (analyser.Executer, error) { return a, nil }
func (a *mockAnalyser) Execute(args []string) (out []byte, err error) {
	if len(args) > 0 && args[0] == "tool" {
		return []byte(`main.go:1: error`), nil
	}
	return nil, nil
}
func (a *mockAnalyser) Stop() error { return nil }

const webhookSecret = "ede9aa6b6e04fafd53f7460fb75644302e249177"

func setup(t *testing.T) (*GitHub, *db.MockDB) {
	var (
		wg sync.WaitGroup
		c  = make(chan interface{})
	)
	queue := queue.NewMemoryQueue(context.Background(), &wg, c)
	g, err := New(&mockAnalyser{}, memDB, queue, 1, integrationKey, webhookSecret)
	return g, memDB
}

func TestWebhookHandler(t *testing.T) {
	tests := []struct {
		signature  string
		event      string
		expectCode int
	}{
		{"sha1=d1e100e3f17e8399b73137382896ff1536c59457", "goci-invalid", http.StatusBadRequest},
		{"sha1=d1e100e3f17e8399b73137382896ff1536c59457", "push", http.StatusOK},
		{"sha1=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "push", http.StatusBadRequest},
	}

	for _, test := range tests {
		g, _ := setup(t)
		body := bytes.NewBufferString(`{"key":"value"}`)
		r, err := http.NewRequest("POST", "https://example.com", body)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Add("X-GitHub-Event", test.event)
		r.Header.Add("X-Hub-Signature", test.signature)
		w := httptest.NewRecorder()
		g.WebHookHandler(w, r)

		if w.Code != test.expectCode {
			t.Fatalf("have code: %v, want: %v, test: %+v", w.Code, test.expectCode, test)
		}
	}
	g, memDB := setup(t)
		accountID      = 3
		senderID       = 4
			ID:    github.Int(senderID),
	want := &db.GHInstallation{
		InstallationID: installationID,
		AccountID:      accountID,
		SenderID:       senderID,
	}

	have, _ := memDB.GetGHInstallation(installationID)
	if !reflect.DeepEqual(have, want) {
		t.Errorf("\nhave: %#v\nwant: %#v", have, want)
	have, _ = memDB.GetGHInstallation(installationID)
	if have != nil {
		t.Errorf("got: %#v, expected nil", have)
	g, memDB := setup(t)
		expectedConfig = analyser.Config{
			BaseURL:    "base-repo-url",
			BaseBranch: "base-branch",
			HeadURL:    "head-repo-url",
			HeadBranch: "head-branch",
		}
		expectedCmtBody = "Name: error"
		expectedCmtSHA  = "error"
		case "/diff-url":
			fmt.Fprintln(w, `diff --git a/subdir/main.go b/subdir/main.go
new file mode 100644
index 0000000..6362395
--- /dev/null
+++ b/main.go
@@ -0,0 +1,1 @@
+var _ = fmt.Sprintln()`)
			fmt.Fprintln(w, "{}")
	expectedConfig.DiffURL = ts.URL + "/diff-url"
		accountID      = 3
		senderID       = 4
	_ = memDB.AddGHInstallation(installationID, accountID, senderID)
	memDB.EnableGHInstallation(installationID)

	memDB.Tools = []db.Tool{
		{Name: "Name", Path: "tool", Args: "-flag %BASE_BRANCH% ./..."},
	}
			DiffURL:     github.String(expectedConfig.DiffURL),
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					CloneURL: github.String(expectedConfig.BaseURL),
				},
				Ref: github.String(expectedConfig.BaseBranch),
			},
					CloneURL: github.String(expectedConfig.HeadURL),
				Ref: github.String(expectedConfig.HeadBranch),
			URL: github.String("repo-url"),
		},
		WebhookCommon: github.WebhookCommon{
			Installation: &github.Installation{
				ID: github.Int(installationID),
	err := g.PullRequestEvent(event)

func TestPullRequestEvent_noInstall(t *testing.T) {
	g, _ := setup(t)

	const installationID = 2
	event := &github.PullRequestEvent{
		Action: github.String("opened"),
		Number: github.Int(1),
		WebhookCommon: github.WebhookCommon{
			Installation: &github.Installation{
				ID: github.Int(installationID),
			},
		},
	}

	err := g.PullRequestEvent(event)
	if want := errors.New("could not find installation with ID 2"); err.Error() != want.Error() {
		t.Errorf("expected error %q have %q", want, err)
	}
}

func TestPullRequestEvent_disabled(t *testing.T) {
	g, memDB := setup(t)

	const installationID = 2

	// Added but not enabled
	_ = memDB.AddGHInstallation(installationID, 3, 4)

	event := &github.PullRequestEvent{
		Action: github.String("opened"),
		Number: github.Int(1),
		WebhookCommon: github.WebhookCommon{
			Installation: &github.Installation{
				ID: github.Int(installationID),
			},
		},
	}

	err := g.PullRequestEvent(event)
	if want := errors.New("could not find installation with ID 2"); err.Error() != want.Error() {
		t.Errorf("expected error %q have %q", want, err)
	}
}