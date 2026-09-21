package service

import (
	"strconv"
	"testing"

	"github.com/adishgithub/adips_backend/internal/dto"
	"github.com/adishgithub/adips_backend/internal/models"
)

func (e *integrationEnv) accountSvc() AccountService {
	return NewAccountService(e.accountRepo, e.txRepo, e.db)
}

// isDeleted reports whether an account row is soft-deleted.
func (e *integrationEnv) isDeleted(t *testing.T, accountID uint) bool {
	t.Helper()

	var account models.Account

	if err := e.db.Unscoped().First(&account, accountID).Error; err != nil {
		t.Fatalf("load account %d: %v", accountID, err)
	}

	return account.DeletedAt.Valid
}

func (e *integrationEnv) liveTransactions(t *testing.T, accountID uint) int64 {
	t.Helper()

	var n int64

	e.db.Model(&models.Transaction{}).Where("account_id = ?", accountID).Count(&n)

	return n
}

// A8, A9 (scenarios 10, 11).
func TestAccountDelete_PlainDeleteRules(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	s := env.newScenario(t)

	empty := env.newAccount(t, s.userID, "Empty", 0)

	// A8: no transactions -> soft delete.
	if err := svc.Delete(s.userID, empty.ID, nil); err != nil {
		t.Fatalf("delete empty account: %v", err)
	}

	if !env.isDeleted(t, empty.ID) {
		t.Error("empty account was not soft-deleted")
	}

	// A2: the name is free again after a delete.
	if _, err := svc.Create(s.userID, dto.CreateAccountRequest{
		Name: "Empty", Type: "bank", Currency: "INR", IconID: 41, ColorID: 2,
	}); err != nil {
		t.Errorf("re-using a deleted account's name must work: %v", err)
	}

	// A9: has transactions, no move target -> 409.
	env.addTx(t, s.userID, s.cash.ID, models.TransactionDirectionDebit, 10)

	requireStatus(t, svc.Delete(s.userID, s.cash.ID, nil), http409)

	if env.isDeleted(t, s.cash.ID) {
		t.Error("account with transactions must not be deleted")
	}
}

// A6, A7 (scenarios 13, 14) for both archive and delete.
func TestAccountLifecycle_DefaultAndLastActiveAreProtected(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	s := env.newScenario(t)

	// A6: the default account.
	_, err := svc.Archive(s.userID, s.main.ID)
	requireStatus(t, err, http409)

	requireStatus(t, svc.Delete(s.userID, s.main.ID, nil), http409)

	// A7: a user with exactly one active account. Archive everything
	// else first, then try to archive / delete the survivor.
	u := env.newUser(t)
	only := env.newAccount(t, u, "Only", 0)
	other := env.newAccount(t, u, "Other", 0)

	// `only` is not the default here, so A6 does not apply to it; A7
	// must be what stops the last active account.
	env.modifyAccount(t, other, func(a *models.Account) { a.IsArchived = true })

	_, err = svc.Archive(u, only.ID)
	requireStatus(t, err, http409)

	requireStatus(t, svc.Delete(u, only.ID, nil), http409)

	// An ARCHIVED account can still be deleted: it is not "active".
	if err := svc.Delete(u, other.ID, nil); err != nil {
		t.Errorf("deleting an archived empty account must work: %v", err)
	}
}

// A11 (scenario 12).
func TestAccountArchive_RequiresZeroBalance(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	s := env.newScenario(t)

	// Post Office holds 50,000.
	_, err := svc.Archive(s.userID, s.post.ID)
	requireStatus(t, err, http409)

	// Move the balance out, then archive succeeds.
	if _, err := env.transferSvc.Create(s.userID, dto.CreateTransferRequest{
		FromAccountID: s.post.ID,
		ToAccountID:   s.main.ID,
		Amount:        50000,
	}); err != nil {
		t.Fatalf("transfer out: %v", err)
	}

	archived, err := svc.Archive(s.userID, s.post.ID)
	if err != nil {
		t.Fatalf("archive after emptying: %v", err)
	}

	if !archived.IsArchived {
		t.Error("response should say is_archived=true")
	}

	// Archiving again is a harmless no-op.
	if _, err := svc.Archive(s.userID, s.post.ID); err != nil {
		t.Errorf("second archive should be idempotent: %v", err)
	}

	// A12: hidden from the default list, visible with include_archived.
	visible, _ := svc.List(s.userID, false)

	for _, a := range visible {
		if a.ID == s.post.ID {
			t.Error("archived account must be hidden from the default list")
		}
	}

	all, _ := svc.List(s.userID, true)

	found := false

	for _, a := range all {
		if a.ID == s.post.ID {
			found = true
		}
	}

	if !found {
		t.Error("archived account must appear with include_archived=true")
	}

	// A12: hidden from the dashboard summary.
	summary, err := svc.Summary(s.userID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}

	for _, a := range summary.Accounts {
		if a.ID == s.post.ID {
			t.Error("archived account must not be on the dashboard")
		}
	}

	// Unarchive is always allowed.
	restored, err := svc.Unarchive(s.userID, s.post.ID)
	if err != nil || restored.IsArchived {
		t.Fatalf("unarchive failed: %+v %v", restored, err)
	}
}

// A12: archived accounts cannot receive transactions or transfers, and
// (extension) their history is read-only so the zero balance holds.
func TestAccountArchive_IsRejectedAsTargetAndFrozen(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	s := env.newScenario(t)

	// Give Post Office some history, then empty and archive it.
	old := env.addTx(t, s.userID, s.post.ID, models.TransactionDirectionDebit, 50000)

	if _, err := svc.Archive(s.userID, s.post.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}

	// New transaction on an archived account -> 400.
	_, err := env.txSvc.Create(s.userID, dto.CreateTransactionRequest{
		Amount: 5, Type: "debit", Category: "Food", CategoryIconID: 10,
		CategoryColorID: 1, Description: "x", Status: "completed",
		PaymentMethod: "cash", Currency: "INR", AccountID: s.post.ID,
	})
	requireStatus(t, err, http400)

	// Moving an existing transaction INTO it -> 400.
	moving := env.addTx(t, s.userID, s.cash.ID, models.TransactionDirectionDebit, 5)

	_, err = env.txSvc.Update(s.userID, moving.ID, dto.UpdateTransactionRequest{AccountID: &s.post.ID})
	requireStatus(t, err, http400)

	// Editing or deleting history on it -> 400 (frozen).
	amount := 1.0

	_, err = env.txSvc.Update(s.userID, old.ID, dto.UpdateTransactionRequest{Amount: &amount})
	requireStatus(t, err, http400)

	requireStatus(t, env.txSvc.Delete(s.userID, old.ID), http400)

	// Moving a transaction OUT of it would also change its balance.
	_, err = env.txSvc.Update(s.userID, old.ID, dto.UpdateTransactionRequest{AccountID: &s.cash.ID})
	requireStatus(t, err, http400)

	// Transfers cannot target it.
	_, err = env.transferSvc.Create(s.userID, dto.CreateTransferRequest{
		FromAccountID: s.main.ID, ToAccountID: s.post.ID, Amount: 1,
	})
	requireStatus(t, err, http400)

	// The archived balance really stayed at zero.
	assertMoney(t, "archived balance", env.balance(t, s.userID, s.post.ID), 0)

	// It cannot be made the default.
	makeDefault := true

	_, err = svc.Update(s.userID, s.post.ID, dto.UpdateAccountRequest{IsDefault: &makeDefault})
	requireStatus(t, err, http409)

	// A12: its history still appears when filtering by that account.
	list, _, err := env.txSvc.List(s.userID, dto.TransactionQuery{
		AccountID: strconv.FormatUint(uint64(s.post.ID), 10),
		Limit:     50,
	})
	if err != nil {
		t.Fatalf("list with account filter: %v", err)
	}

	seen := false

	for _, row := range list {
		if row.ID == old.ID && row.AccountName == "Post Office" {
			seen = true
		}
	}

	if !seen {
		t.Error("history of an archived account must stay visible")
	}
}

// Scenario 15 + 8.5: merge-delete keeps total money identical, and
// collapses transfers between the two accounts.
func TestAccountMergeDelete_KeepsTotalMoneyAndCollapsesTransfers(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	s := env.newScenario(t)

	// Source = Cash (opening 5,000), target = Main (opening 80,000).
	env.addTx(t, s.userID, s.cash.ID, models.TransactionDirectionDebit, 1200)
	env.addTx(t, s.userID, s.cash.ID, models.TransactionDirectionCredit, 300)
	env.addTx(t, s.userID, s.main.ID, models.TransactionDirectionDebit, 700)

	// Two transfers between source and target, one in each direction.
	for _, req := range []dto.CreateTransferRequest{
		{FromAccountID: s.main.ID, ToAccountID: s.cash.ID, Amount: 2000},
		{FromAccountID: s.cash.ID, ToAccountID: s.main.ID, Amount: 400},
	} {
		if _, err := env.transferSvc.Create(s.userID, req); err != nil {
			t.Fatalf("transfer: %v", err)
		}
	}

	// A transfer to a THIRD account must survive the merge.
	surviving, err := env.transferSvc.Create(s.userID, dto.CreateTransferRequest{
		FromAccountID: s.cash.ID, ToAccountID: s.post.ID, Amount: 100,
	})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}

	// A pending and a future-dated row must move along without being
	// counted (T5, T6).
	pending := env.addTx(t, s.userID, s.cash.ID, models.TransactionDirectionDebit, 999)
	env.db.Model(pending).Update("status", "pending")

	sumBefore := env.allBalancesSum(t, s.userID)
	sourceBefore := env.balance(t, s.userID, s.cash.ID)
	targetBefore := env.balance(t, s.userID, s.main.ID)
	postBefore := env.balance(t, s.userID, s.post.ID)

	// The preview must predict the result.
	preview, err := svc.DeletePreview(s.userID, s.cash.ID, s.main.ID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	assertMoney(t, "preview source balance", preview.SourceCurrentBalance, sourceBefore)
	assertMoney(t, "preview target before", preview.TargetBalanceBefore, targetBefore)
	assertMoney(t, "preview target after", preview.TargetBalanceAfter, targetBefore+sourceBefore)

	if preview.CollapsedTransferCount != 2 {
		t.Errorf("collapsed transfers = %d, want 2", preview.CollapsedTransferCount)
	}

	// Cash rows: 1200, 300, 999(pending), 2000 leg, 400 leg, 100 leg = 6.
	// Collapsing legs (2 transfers) = 2 -> 4 are moved and kept.
	if preview.MovedTransactionCount != 4 {
		t.Errorf("moved transactions = %d, want 4", preview.MovedTransactionCount)
	}

	// The preview itself must not change anything.
	if env.isDeleted(t, s.cash.ID) || env.liveTransactions(t, s.cash.ID) != 6 {
		t.Fatal("preview modified data")
	}

	// --- do it
	if err := svc.Delete(s.userID, s.cash.ID, &s.main.ID); err != nil {
		t.Fatalf("merge-delete: %v", err)
	}

	if !env.isDeleted(t, s.cash.ID) {
		t.Error("source account should be soft-deleted")
	}

	// THE invariant of section 8.5.
	assertMoney(t, "sum of all balances", env.allBalancesSum(t, s.userID), sumBefore)
	assertMoney(t, "target after merge", env.balance(t, s.userID, s.main.ID), targetBefore+sourceBefore)
	assertMoney(t, "target matches the preview", env.balance(t, s.userID, s.main.ID), preview.TargetBalanceAfter)
	assertMoney(t, "unrelated account", env.balance(t, s.userID, s.post.ID), postBefore)

	// Nothing may still point at the deleted account.
	if n := env.liveTransactions(t, s.cash.ID); n != 0 {
		t.Errorf("%d live transaction(s) still on the deleted account", n)
	}

	// The source->target transfers collapsed; the third-account one lives on.
	var groups int64

	env.db.Model(&models.Transaction{}).
		Where("transfer_group_id IS NOT NULL").
		Distinct("transfer_group_id").
		Count(&groups)

	if groups != 1 {
		t.Errorf("live transfers after merge = %d, want 1", groups)
	}

	got, err := env.transferSvc.GetByGroup(s.userID, surviving.TransferGroupID)
	if err != nil {
		t.Fatalf("surviving transfer must remain readable: %v", err)
	}

	if got.Debit.AccountID != s.main.ID || got.Credit.AccountID != s.post.ID {
		t.Errorf("surviving transfer not re-pointed to the target: %+v", got)
	}
}

// Merge-delete also works for an ARCHIVED source and carries its
// opening balance.
func TestAccountMergeDelete_ArchivedSourceAndOpeningBalance(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	s := env.newScenario(t)

	before := env.allBalancesSum(t, s.userID)

	env.modifyAccount(t, s.post, func(a *models.Account) { a.IsArchived = true })

	if err := svc.Delete(s.userID, s.post.ID, &s.main.ID); err != nil {
		t.Fatalf("merge archived source: %v", err)
	}

	assertMoney(t, "sum unchanged", env.allBalancesSum(t, s.userID), before)
	assertMoney(t, "opening balance carried", env.balance(t, s.userID, s.main.ID), 130000)
}

// Scenario 16: invalid merge targets.
func TestAccountMergeDelete_InvalidTargets(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	s := env.newScenario(t)

	env.addTx(t, s.userID, s.post.ID, models.TransactionDirectionDebit, 10)

	usd := env.newAccount(t, s.userID, "Dollar", 0)
	env.modifyAccount(t, usd, func(a *models.Account) { a.Currency = "USD" })

	archived := env.newAccount(t, s.userID, "Old", 0)
	env.modifyAccount(t, archived, func(a *models.Account) { a.IsArchived = true })

	stranger := env.newUser(t)
	foreign := env.newAccount(t, stranger, "Foreign", 0)

	tests := []struct {
		name   string
		target uint
		status int
	}{
		{"same account", s.post.ID, http400},
		{"archived target", archived.ID, http400},
		{"different currency", usd.ID, http400},
		{"another user's account (A1)", foreign.ID, http404},
		{"missing account", 999999, http404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.target

			requireStatus(t, svc.Delete(s.userID, s.post.ID, &target), tt.status)

			_, err := svc.DeletePreview(s.userID, s.post.ID, target)
			requireStatus(t, err, tt.status)
		})
	}

	// Nothing was deleted or moved by the failed attempts.
	if env.isDeleted(t, s.post.ID) || env.liveTransactions(t, s.post.ID) != 1 {
		t.Error("a rejected merge changed data")
	}
}

// A6/A7 also guard the merge-delete source.
func TestAccountMergeDelete_SourceRules(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	s := env.newScenario(t)

	// Default account as the source -> 409 (A6).
	requireStatus(t, svc.Delete(s.userID, s.main.ID, &s.cash.ID), http409)

	_, err := svc.DeletePreview(s.userID, s.main.ID, s.cash.ID)
	requireStatus(t, err, http409)
}

// A1 (scenario 17): every lifecycle operation treats another user's
// account as not found.
func TestAccountLifecycle_OwnershipIsolation(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	s := env.newScenario(t)

	stranger := env.newUser(t)
	mine := env.newAccount(t, stranger, "Mine", 0)

	actual := 10.0

	_, err := svc.Archive(stranger, s.cash.ID)
	requireStatus(t, err, http404)

	_, err = svc.Unarchive(stranger, s.cash.ID)
	requireStatus(t, err, http404)

	requireStatus(t, svc.Delete(stranger, s.cash.ID, nil), http404)

	_, err = svc.DeletePreview(stranger, s.cash.ID, mine.ID)
	requireStatus(t, err, http404)

	_, err = svc.Adjust(stranger, s.cash.ID, dto.AdjustAccountRequest{ActualBalance: &actual})
	requireStatus(t, err, http404)

	// Reordering someone else's account fails the whole batch.
	err = svc.Reorder(stranger, dto.ReorderAccountsRequest{
		Items: []dto.ReorderItem{{ID: mine.ID, SortOrder: 5}, {ID: s.cash.ID, SortOrder: 6}},
	})
	requireStatus(t, err, http400)

	// ...and rolled back: the stranger's own account was not touched.
	var reloaded models.Account

	env.db.First(&reloaded, mine.ID)

	if reloaded.SortOrder != 0 {
		t.Errorf("failed batch was not rolled back: sort_order=%d", reloaded.SortOrder)
	}
}

func TestAccountReorder(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	s := env.newScenario(t)

	err := svc.Reorder(s.userID, dto.ReorderAccountsRequest{
		Items: []dto.ReorderItem{
			{ID: s.post.ID, SortOrder: 0},
			{ID: s.main.ID, SortOrder: 1},
			{ID: s.cash.ID, SortOrder: 2},
			{ID: s.business.ID, SortOrder: 3},
		},
	})
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}

	list, err := svc.List(s.userID, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := []uint{s.post.ID, s.main.ID, s.cash.ID, s.business.ID}

	for i, id := range want {
		if list[i].ID != id {
			t.Fatalf("position %d = account %d, want %d", i, list[i].ID, id)
		}
	}
}

// Section 8.6.
func TestAccountAdjust(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	s := env.newScenario(t)

	// Cash is 5,000; the wallet really holds 3,500 -> debit 1,500.
	actual := 3500.0

	res, err := svc.Adjust(s.userID, s.cash.ID, dto.AdjustAccountRequest{
		ActualBalance: &actual, Note: "counted the wallet",
	})
	if err != nil {
		t.Fatalf("adjust down: %v", err)
	}

	assertMoney(t, "difference", res.Difference, -1500)
	assertMoney(t, "current balance", res.Account.CurrentBalance, 3500)
	assertMoney(t, "stored balance", env.balance(t, s.userID, s.cash.ID), 3500)

	adj := res.Adjustment

	if adj == nil || adj.Type != "debit" || adj.Amount != 1500 ||
		adj.Category != "Balance Adjustment" || adj.Status != "completed" ||
		adj.AccountID != s.cash.ID || adj.Note != "counted the wallet" ||
		adj.Currency != "INR" {
		t.Errorf("adjustment transaction wrong: %+v", adj)
	}

	// Up: a credit.
	actual = 4000

	res, err = svc.Adjust(s.userID, s.cash.ID, dto.AdjustAccountRequest{ActualBalance: &actual})
	if err != nil {
		t.Fatalf("adjust up: %v", err)
	}

	if res.Adjustment == nil || res.Adjustment.Type != "credit" || res.Adjustment.Amount != 500 {
		t.Errorf("expected a 500 credit, got %+v", res.Adjustment)
	}

	assertMoney(t, "balance after up", env.balance(t, s.userID, s.cash.ID), 4000)

	// Already correct: nothing is written.
	before := env.liveTransactions(t, s.cash.ID)

	res, err = svc.Adjust(s.userID, s.cash.ID, dto.AdjustAccountRequest{ActualBalance: &actual})
	if err != nil {
		t.Fatalf("adjust no-op: %v", err)
	}

	if res.Adjustment != nil || res.Difference != 0 || env.liveTransactions(t, s.cash.ID) != before {
		t.Errorf("no-op adjust must not create a transaction: %+v", res)
	}

	// Zero is a valid actual balance, and negative is allowed (D5).
	zero := 0.0

	if _, err := svc.Adjust(s.userID, s.cash.ID, dto.AdjustAccountRequest{ActualBalance: &zero}); err != nil {
		t.Fatalf("adjust to zero: %v", err)
	}

	assertMoney(t, "balance at zero", env.balance(t, s.userID, s.cash.ID), 0)

	negative := -250.0

	if _, err := svc.Adjust(s.userID, s.cash.ID, dto.AdjustAccountRequest{ActualBalance: &negative}); err != nil {
		t.Fatalf("adjust to negative: %v", err)
	}

	assertMoney(t, "balance negative", env.balance(t, s.userID, s.cash.ID), -250)

	// Archived accounts cannot be adjusted (A12).
	env.modifyAccount(t, s.post, func(a *models.Account) { a.IsArchived = true })

	_, err = svc.Adjust(s.userID, s.post.ID, dto.AdjustAccountRequest{ActualBalance: &actual})
	requireStatus(t, err, http400)
}

// A13 (scenario 3) + the Phase 1 bug fix: include_in_total defaults to
// true when omitted, and false only when explicitly sent.
func TestAccountCreate_IncludeInTotalDefaultsToTrue(t *testing.T) {
	env := newIntegrationEnv(t)
	svc := env.accountSvc()
	userID := env.newUser(t)

	created, err := svc.Create(userID, dto.CreateAccountRequest{
		Name: "Default", Type: "bank", Currency: "INR", IconID: 41, ColorID: 2,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if !created.IncludeInTotal {
		t.Error("omitted include_in_total must default to true")
	}

	off := false

	created, err = svc.Create(userID, dto.CreateAccountRequest{
		Name: "Excluded", Type: "business", Currency: "INR", IconID: 41, ColorID: 2,
		IncludeInTotal: &off,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if created.IncludeInTotal {
		t.Error("explicit include_in_total=false must be respected")
	}
}

// Scenario 3: crediting an account that is excluded from the total
// leaves the total unchanged.
func TestAccountTotals_ExcludedAccountDoesNotAffectTotal(t *testing.T) {
	env := newIntegrationEnv(t)
	s := env.newScenario(t)

	before := env.totalBalance(t, s.userID)

	env.addTx(t, s.userID, s.business.ID, models.TransactionDirectionCredit, 15000)

	assertMoney(t, "business balance", env.balance(t, s.userID, s.business.ID), 55000)
	assertMoney(t, "personal total", env.totalBalance(t, s.userID), before)
}
