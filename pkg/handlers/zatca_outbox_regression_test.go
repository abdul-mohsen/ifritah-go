package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	db "ifritah/web-service-gin/pkg/db/gen"
	"ifritah/web-service-gin/pkg/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
)

type regressionJetStream struct {
	nats.JetStreamContext
	publishErr error
	calls      int
}

func (s *regressionJetStream) Publish(string, []byte, ...nats.PubOpt) (*nats.PubAck, error) {
	s.calls++
	if s.publishErr != nil {
		return nil, s.publishErr
	}
	return &nats.PubAck{}, nil
}

func newRegressionPublisher(stream *regressionJetStream) *ZatcaPublisher {
	return &ZatcaPublisher{
		js:     stream,
		dbName: "test_tenant",
	}
}

func runZatcaOutboxRequest(
	t *testing.T,
	handlerFunc gin.HandlerFunc,
	method, path, body string,
) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("decoded_jwt", &model.Claims{Id: 7, Username: "admin"})
	if strings.Contains(path, "/branch/") {
		c.Params = gin.Params{{Key: "id", Value: "7"}}
	}

	handlerFunc(c)
	return w
}

func TestOnboardBranchZatcaEnqueuesBeforePublication(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	stream := &regressionJetStream{}
	h := New(sqlDB, db.New(sqlDB), newRegressionPublisher(stream))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE branch_zatca_config").
		WithArgs(uint32(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO zatca_event_outbox").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := runZatcaOutboxRequest(
		t,
		h.OnboardBranchZatca,
		http.MethodPost,
		"/api/v2/branch/7/zatca/onboard",
		`{"otp":"123456"}`,
	)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if stream.calls != 0 {
		t.Fatalf("publisher calls = %d, want 0 before the request transaction commits", stream.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("transactional outbox expectations: %v", err)
	}
}

func TestAddBillEnqueuesBeforePublication(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	stream := &regressionJetStream{}
	h := New(sqlDB, db.New(sqlDB), newRegressionPublisher(stream))

	mock.ExpectQuery("select store.id from store").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(54))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO branch_sequence").
		WithArgs(uint32(7), "bill").
		WillReturnResult(sqlmock.NewResult(101, 1))
	mock.ExpectExec("insert into bill").
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec("UPDATE bill").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT COALESCE").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("disable"))
	mock.ExpectExec("INSERT INTO zatca_event_outbox").
		WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()

	w := runZatcaOutboxRequest(
		t,
		h.AddBill,
		http.MethodPost,
		"/api/v2/bill",
		`{
			"store_id":54,
			"state":1,
			"discount":"0",
			"maintenance_cost":"0",
			"payment_method":10,
			"branch_id":7,
			"products":[],
			"manual_products":[]
		}`,
	)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusCreated, w.Body.String())
	}
	if stream.calls != 0 {
		t.Fatalf("publisher calls = %d, want 0 before the request transaction commits", stream.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("transactional outbox expectations: %v", err)
	}
}

func TestCreditBillEnqueuesBeforePublicationFailure(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	stream := &regressionJetStream{publishErr: errPublisherUnavailable}
	h := New(sqlDB, db.New(sqlDB), newRegressionPublisher(stream))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO credit_note").
		WithArgs(uint64(99), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(123, 1))
	mock.ExpectQuery("SELECT COALESCE").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("disable"))
	mock.ExpectQuery("SELECT branches.id as branch_id").
		WithArgs(uint64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"branch_id"}).AddRow(7))
	mock.ExpectExec("INSERT INTO zatca_event_outbox").
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	w := runZatcaOutboxRequest(
		t,
		h.CreditBill,
		http.MethodPost,
		"/api/v2/bill/credit",
		`{"bill_id":99,"note":"returned"}`,
	)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d or %d; body=%s", w.Code, http.StatusCreated, http.StatusOK, w.Body.String())
	}
	if stream.calls != 0 {
		t.Fatalf("publisher calls = %d, want 0 before the request transaction commits", stream.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("transactional outbox expectations: %v", err)
	}
}

var errPublisherUnavailable = &publisherUnavailableError{}

type publisherUnavailableError struct{}

func (*publisherUnavailableError) Error() string { return "publisher unavailable" }
