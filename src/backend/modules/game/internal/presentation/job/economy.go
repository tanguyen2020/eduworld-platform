package job

import "context"

// WalletReconcile is the name of the nightly wallet reconciliation job (F075).
const WalletReconcile = "wallet-reconcile"

// WalletReconciler is the use case behind wallet-reconcile (command.Economy).
type WalletReconciler interface {
	ReconcileWallets(ctx context.Context, batch int) (int, error)
}

// WalletReconcileJob is one run of the nightly wallet-reconcile (F075-AC-03): wallets whose balance differs from Σ ledger
// are reported (metric, audit), never corrected.
func WalletReconcileJob(r WalletReconciler) RunOnce {
	return func(ctx context.Context) (int, error) { return r.ReconcileWallets(ctx, Batch) }
}
