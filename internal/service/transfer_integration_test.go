package service

import (
	"testing"
	"time"

	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
)

// Scenario 2 + X5 + X7 + D3:
// an ATM withdrawal moves balances between two accounts without
// changing the total or the monthly expense.
func TestTransfer_ATMWithdrawal(t *testing.T) {
	env := newIntegrationEnv(t)
	s := env.newScenario(t)

	// A normal expense first, so the summary has something to count.
	env.addTx(t, s.userID, s.cash.ID, models.TransactionDirectionDebit, 1200)

	totalBefore := env.totalBalance(t, s.userID)

	summaryBefore, err := env.txSvc.Summary(s.userID, dto.TransactionQuery{})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}

	res, err := env.transferSvc.Create(s.userID, dto.CreateTransferRequest{
		FromAccountID: s.main.ID,
		ToAccountID:   s.cash.ID,
		Amount:        2000,
		Note:          "ATM",
	})
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}

	// Balances moved: Main 80,000 - 2,000 ; Cash 5,000 - 1,200 + 2,000.
	assertMoney(t, "main balance", env.balance(t, s.userID, s.main.ID), 78000)
	assertMoney(t, "cash balance", env.balance(t, s.userID, s.cash.ID), 5800)

	// Total unchanged.
	assertMoney(t, "total balance", env.totalBalance(t, s.userID), totalBefore)

	// X5: two legs, one group, fixed category values.
	if res.TransferGroupID == "" {
		t.Fatal("transfer_group_id is empty")
	}

	d, c := res.Debit, res.Credit

	if d.TransferGroupID == nil || c.TransferGroupID == nil ||
		*d.TransferGroupID != res.TransferGroupID ||
		*c.TransferGroupID != res.TransferGroupID {
		t.Errorf("legs do not share transfer_group_id %s", res.TransferGroupID)
	}

	if d.Type != "debit" || d.AccountID != s.main.ID || d.AccountName != "Main Account" {
		t.Errorf("debit leg wrong: %+v", d)
	}

	if c.Type != "credit" || c.AccountID != s.cash.ID || c.AccountName != "Cash" {
		t.Errorf("credit leg wrong: %+v", c)
	}

	for _, leg := range []dto.TransactionResponse{d, c} {
		if leg.Category != "Transfer" || leg.CategoryIconID != 30 || leg.CategoryColorID != 1 {
			t.Errorf("leg category snapshot wrong: %+v", leg)
		}

		if leg.Description != "Transfer" || leg.PaymentMethod != "transfer" ||
			leg.Status != "completed" || leg.Amount != 2000 || leg.Note != "ATM" ||
			leg.Currency != "INR" {
			t.Errorf("leg fields wrong: %+v", leg)
		}
	}

	if !d.TransactionDate.Equal(c.TransactionDate) {
		t.Error("legs must share transaction_date")
	}

	// X7: transfers are not income or expense.
	summaryAfter, err := env.txSvc.Summary(s.userID, dto.TransactionQuery{})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}

	assertMoney(t, "monthly expense", summaryAfter.TotalDebit, summaryBefore.TotalDebit)
	assertMoney(t, "monthly income", summaryAfter.TotalCredit, summaryBefore.TotalCredit)

	if summaryAfter.Count != summaryBefore.Count {
		t.Errorf("summary count = %d, want %d", summaryAfter.Count, summaryBefore.Count)
	}

	// X7: include_transfers=true counts both legs.
	withTransfers, err := env.txSvc.Summary(s.userID, dto.TransactionQuery{IncludeTransfers: true})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}

	assertMoney(t, "debit incl. transfers", withTransfers.TotalDebit, 1200+2000)
	assertMoney(t, "credit incl. transfers", withTransfers.TotalCredit, 2000)

	if withTransfers.Count != 3 {
		t.Errorf("count incl. transfers = %d, want 3", withTransfers.Count)
	}

	// X7: the list DOES return transfer legs.
	list, _, err := env.txSvc.List(s.userID, dto.TransactionQuery{Limit: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	legs := 0

	for _, row := range list {
		if row.TransferGroupID != nil {
			legs++
		}
	}

	if len(list) != 3 || legs != 2 {
		t.Errorf("list has %d rows / %d transfer legs, want 3 / 2", len(list), legs)
	}
}

// X1, X2, X3, X4 (scenario 8 and friends).
func TestTransfer_ValidationRules(t *testing.T) {
	env := newIntegrationEnv(t)
	s := env.newScenario(t)

	stranger := env.newUser(t)
	foreign := env.newAccount(t, stranger, "Foreign", 100)

	usd := env.newAccount(t, s.userID, "Dollar", 0)
	env.modifyAccount(t, usd, func(a *models.Account) { a.Currency = "USD" })

	archived := env.newAccount(t, s.userID, "Old", 0)
	env.modifyAccount(t, archived, func(a *models.Account) { a.IsArchived = true })

	base := dto.CreateTransferRequest{
		FromAccountID: s.main.ID,
		ToAccountID:   s.cash.ID,
		Amount:        100,
	}

	tests := []struct {
		name   string
		mutate func(*dto.CreateTransferRequest)
		status int
	}{
		{"X1 same account", func(r *dto.CreateTransferRequest) { r.ToAccountID = r.FromAccountID }, http400},
		{"X2 destination is another user's", func(r *dto.CreateTransferRequest) { r.ToAccountID = foreign.ID }, http404},
		{"X2 source is another user's", func(r *dto.CreateTransferRequest) { r.FromAccountID = foreign.ID }, http404},
		{"X2 destination does not exist", func(r *dto.CreateTransferRequest) { r.ToAccountID = 999999 }, http404},
		{"X2 archived destination", func(r *dto.CreateTransferRequest) { r.ToAccountID = archived.ID }, http400},
		{"X2 archived source", func(r *dto.CreateTransferRequest) { r.FromAccountID = archived.ID }, http400},
		{"X3 different currency", func(r *dto.CreateTransferRequest) { r.ToAccountID = usd.ID }, http400},
		{"X4 zero amount", func(r *dto.CreateTransferRequest) { r.Amount = 0 }, http400},
		{"X4 negative amount", func(r *dto.CreateTransferRequest) { r.Amount = -5 }, http400},
		{"X4 rounds to zero", func(r *dto.CreateTransferRequest) { r.Amount = 0.001 }, http400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := base
			tt.mutate(&req)

			_, err := env.transferSvc.Create(s.userID, req)
			requireStatus(t, err, tt.status)
		})
	}

	// Nothing above may have written a row.
	var rows int64

	env.db.Model(&models.Transaction{}).Count(&rows)

	if rows != 0 {
		t.Errorf("rejected transfers left %d transaction row(s)", rows)
	}
}

// X8 / D5 (scenario 9): overdraft is allowed.
func TestTransfer_OverdraftAllowed(t *testing.T) {
	env := newIntegrationEnv(t)
	s := env.newScenario(t)

	_, err := env.transferSvc.Create(s.userID, dto.CreateTransferRequest{
		FromAccountID: s.cash.ID,
		ToAccountID:   s.main.ID,
		Amount:        7000, // Cash only holds 5,000
	})
	if err != nil {
		t.Fatalf("overdraft transfer must succeed: %v", err)
	}

	assertMoney(t, "cash goes negative", env.balance(t, s.userID, s.cash.ID), -2000)
	assertMoney(t, "main", env.balance(t, s.userID, s.main.ID), 87000)
}

// X6 + scenario 7: edit and delete act on BOTH legs.
func TestTransfer_UpdateAndDeleteAffectBothLegs(t *testing.T) {
	env := newIntegrationEnv(t)
	s := env.newScenario(t)

	created, err := env.transferSvc.Create(s.userID, dto.CreateTransferRequest{
		FromAccountID: s.main.ID,
		ToAccountID:   s.cash.ID,
		Amount:        2000,
		Note:          "ATM",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	group := created.TransferGroupID

	// --- amount + note + date on both legs
	newAmount := 500.0
	newNote := "smaller"
	newDate := time.Now().Add(-48 * time.Hour).UTC().Truncate(time.Second)

	updated, err := env.transferSvc.Update(s.userID, group, dto.UpdateTransferRequest{
		Amount:          &newAmount,
		Note:            &newNote,
		TransactionDate: &newDate,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	for _, leg := range []dto.TransactionResponse{updated.Debit, updated.Credit} {
		if leg.Amount != 500 || leg.Note != "smaller" || !leg.TransactionDate.Equal(newDate) {
			t.Errorf("leg not updated: %+v", leg)
		}
	}

	assertMoney(t, "main after amount edit", env.balance(t, s.userID, s.main.ID), 79500)
	assertMoney(t, "cash after amount edit", env.balance(t, s.userID, s.cash.ID), 5500)

	// --- clearing the note really clears it (map update, not Save)
	empty := ""

	cleared, err := env.transferSvc.Update(s.userID, group, dto.UpdateTransferRequest{Note: &empty})
	if err != nil {
		t.Fatalf("clear note: %v", err)
	}

	if cleared.Debit.Note != "" || cleared.Credit.Note != "" {
		t.Errorf("note not cleared: %q / %q", cleared.Debit.Note, cleared.Credit.Note)
	}

	// --- change the destination to another account
	updated, err = env.transferSvc.Update(s.userID, group, dto.UpdateTransferRequest{
		ToAccountID: &s.post.ID,
	})
	if err != nil {
		t.Fatalf("move destination: %v", err)
	}

	if updated.Credit.AccountID != s.post.ID || updated.Credit.AccountName != "Post Office" {
		t.Errorf("credit leg not moved: %+v", updated.Credit)
	}

	assertMoney(t, "cash restored", env.balance(t, s.userID, s.cash.ID), 5000)
	assertMoney(t, "post office", env.balance(t, s.userID, s.post.ID), 50500)

	// --- X1 on update: cannot make source == destination
	_, err = env.transferSvc.Update(s.userID, group, dto.UpdateTransferRequest{
		ToAccountID: &s.main.ID,
	})
	requireStatus(t, err, http400)

	// --- X3 on update: cannot move a leg to a different currency
	usd := env.newAccount(t, s.userID, "Dollar", 0)
	env.modifyAccount(t, usd, func(a *models.Account) { a.Currency = "USD" })

	_, err = env.transferSvc.Update(s.userID, group, dto.UpdateTransferRequest{
		ToAccountID: &usd.ID,
	})
	requireStatus(t, err, http400)

	// A rejected update must not have changed anything.
	assertMoney(t, "main unchanged by rejected edits", env.balance(t, s.userID, s.main.ID), 79500)

	// --- delete removes both legs and restores both balances (#7)
	if err := env.transferSvc.Delete(s.userID, group); err != nil {
		t.Fatalf("delete: %v", err)
	}

	assertMoney(t, "main restored", env.balance(t, s.userID, s.main.ID), 80000)
	assertMoney(t, "post restored", env.balance(t, s.userID, s.post.ID), 50000)

	var live int64

	env.db.Model(&models.Transaction{}).Count(&live)

	if live != 0 {
		t.Errorf("%d live leg(s) remain after delete", live)
	}

	_, err = env.transferSvc.GetByGroup(s.userID, group)
	requireStatus(t, err, http404)
}

// A1 (scenario 17): another user's transfer behaves as not found.
func TestTransfer_OwnershipIsolation(t *testing.T) {
	env := newIntegrationEnv(t)
	s := env.newScenario(t)

	created, err := env.transferSvc.Create(s.userID, dto.CreateTransferRequest{
		FromAccountID: s.main.ID,
		ToAccountID:   s.cash.ID,
		Amount:        100,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	stranger := env.newUser(t)
	group := created.TransferGroupID
	amount := 1.0

	_, err = env.transferSvc.GetByGroup(stranger, group)
	requireStatus(t, err, http404)

	_, err = env.transferSvc.Update(stranger, group, dto.UpdateTransferRequest{Amount: &amount})
	requireStatus(t, err, http404)

	requireStatus(t, env.transferSvc.Delete(stranger, group), http404)

	// The owner's transfer is untouched.
	got, err := env.transferSvc.GetByGroup(s.userID, group)
	if err != nil || got.Debit.Amount != 100 {
		t.Fatalf("owner's transfer damaged: %+v %v", got, err)
	}
}

// T4 (scenario 6): transfer legs cannot be edited or deleted through
// /transactions.
func TestTransfer_LegsProtectedFromTransactionEndpoints(t *testing.T) {
	env := newIntegrationEnv(t)
	s := env.newScenario(t)

	created, err := env.transferSvc.Create(s.userID, dto.CreateTransferRequest{
		FromAccountID: s.main.ID,
		ToAccountID:   s.cash.ID,
		Amount:        100,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	for _, leg := range []dto.TransactionResponse{created.Debit, created.Credit} {
		amount := 1.0

		_, err := env.txSvc.Update(s.userID, leg.ID, dto.UpdateTransactionRequest{Amount: &amount})
		requireStatus(t, err, http409)

		requireStatus(t, env.txSvc.Delete(s.userID, leg.ID), http409)
	}

	assertMoney(t, "main untouched", env.balance(t, s.userID, s.main.ID), 79900)
}

// Extension of A12: archived accounts are read-only for transfers, so
// an archived account can never drift away from the zero balance that
// A11 required.
func TestTransfer_ArchivedAccountsAreReadOnly(t *testing.T) {
	env := newIntegrationEnv(t)
	s := env.newScenario(t)

	created, err := env.transferSvc.Create(s.userID, dto.CreateTransferRequest{
		FromAccountID: s.main.ID,
		ToAccountID:   s.cash.ID,
		Amount:        100,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Archive the cash account after the fact (Phase 3 adds the real
	// endpoint; here we just flip the flag).
	env.modifyAccount(t, s.cash, func(a *models.Account) { a.IsArchived = true })

	amount := 1.0

	_, err = env.transferSvc.Update(s.userID, created.TransferGroupID, dto.UpdateTransferRequest{Amount: &amount})
	requireStatus(t, err, http400)

	requireStatus(t, env.transferSvc.Delete(s.userID, created.TransferGroupID), http400)

	// Reading is still allowed: archived accounts stay visible in history.
	if _, err := env.transferSvc.GetByGroup(s.userID, created.TransferGroupID); err != nil {
		t.Fatalf("get must still work: %v", err)
	}
}
