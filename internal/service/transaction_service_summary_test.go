package service

import (
	"testing"

	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
	"github.com/adishgithub/adips_backend/internal/utils"
	"gorm.io/gorm"
)

// fakeTransactionRepo satisfies repository.TransactionRepository and
// records the query the service passes to Summary, so the status
// defaulting rule can be tested without a database.
type fakeTransactionRepo struct {
	gotSummaryQuery dto.TransactionQuery
	gotSummaryUser  uint
}

func (f *fakeTransactionRepo) Create(tx *models.Transaction) error { return nil }
func (f *fakeTransactionRepo) FindByID(id uint) (*models.Transaction, error) {
	return nil, nil
}
func (f *fakeTransactionRepo) Update(tx *models.Transaction) error { return nil }
func (f *fakeTransactionRepo) Delete(id uint) error                { return nil }
func (f *fakeTransactionRepo) List(userID uint, q dto.TransactionQuery, p utils.Pagination, s utils.Sort) ([]models.Transaction, int64, error) {
	return nil, 0, nil
}
func (f *fakeTransactionRepo) Summary(userID uint, q dto.TransactionQuery) (dto.SummaryResponse, error) {
	f.gotSummaryUser = userID
	f.gotSummaryQuery = q
	return dto.SummaryResponse{}, nil
}

// fakeAccountRepo satisfies repository.AccountRepository. The Summary
// tests below never touch accounts, so every method is a no-op stub.
type fakeAccountRepo struct{}

func (f *fakeAccountRepo) Create(account *models.Account) error { return nil }
func (f *fakeAccountRepo) FindByID(id uint) (*models.Account, error) {
	return nil, nil
}
func (f *fakeAccountRepo) FindByUserAndID(userID, id uint) (*models.Account, error) {
	return nil, nil
}
func (f *fakeAccountRepo) FindAllByUser(userID uint, includeArchived bool) ([]models.Account, error) {
	return nil, nil
}
func (f *fakeAccountRepo) Update(account *models.Account) error { return nil }
func (f *fakeAccountRepo) CountTransactions(accountID uint) (int64, error) {
	return 0, nil
}
func (f *fakeAccountRepo) BalancesByUser(userID uint) (map[uint]float64, error) {
	return nil, nil
}
func (f *fakeAccountRepo) BalanceByID(userID, accountID uint) (float64, error) {
	return 0, nil
}
func (f *fakeAccountRepo) ClearDefaultExcept(tx *gorm.DB, userID, accountID uint) error {
	return nil
}

func TestSummaryStatusDefaulting(t *testing.T) {
	tests := []struct {
		name       string
		inStatus   string
		wantStatus string
	}{
		{"no status defaults to completed", "", "completed"},
		{"explicit pending is respected", "pending", "pending"},
		{"explicit failed is respected", "failed", "failed"},
		{"explicit completed is respected", "completed", "completed"},
		{"all removes the status filter", "all", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeTransactionRepo{}
			svc := NewTransactionService(repo, &fakeAccountRepo{})

			_, err := svc.Summary(42, dto.TransactionQuery{Status: tt.inStatus})
			if err != nil {
				t.Fatalf("Summary returned error: %v", err)
			}
			if repo.gotSummaryQuery.Status != tt.wantStatus {
				t.Errorf("status passed to repo = %q, want %q", repo.gotSummaryQuery.Status, tt.wantStatus)
			}
			if repo.gotSummaryUser != 42 {
				t.Errorf("user id passed to repo = %d, want 42", repo.gotSummaryUser)
			}
		})
	}
}

// The defaulting must only touch Status; every other filter has to
// reach the repository untouched.
func TestSummaryKeepsOtherFilters(t *testing.T) {
	repo := &fakeTransactionRepo{}
	svc := NewTransactionService(repo, &fakeAccountRepo{})

	in := dto.TransactionQuery{
		Type:          "debit",
		Category:      "Food",
		PaymentMethod: "upi",
		Currency:      "INR",
		StartDate:     "2026-09-01",
		EndDate:       "2026-09-30",
	}
	if _, err := svc.Summary(1, in); err != nil {
		t.Fatalf("Summary returned error: %v", err)
	}

	got := repo.gotSummaryQuery
	want := in
	want.Status = "completed"
	if got != want {
		t.Errorf("query passed to repo = %+v, want %+v", got, want)
	}
}

// The caller's query struct is passed by value; defaulting must not
// mutate anything the caller still holds.
func TestWithDefaultSummaryStatusDoesNotMutateInput(t *testing.T) {
	in := dto.TransactionQuery{Status: ""}
	_ = withDefaultSummaryStatus(in)
	if in.Status != "" {
		t.Errorf("input mutated: Status = %q", in.Status)
	}
}
