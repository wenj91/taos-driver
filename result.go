package taos

type TaosResult struct {
	LId int64
	RAf int64
}

func (tr *TaosResult) LastInsertId() (int64, error) {
	return tr.LId, nil
}

func (tr *TaosResult) RowsAffected() (int64, error) {
	return tr.RAf, nil
}
