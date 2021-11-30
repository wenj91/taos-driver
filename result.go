package taos

type taosResult struct {
	LId int64
	RAf int64
}

func (tr *taosResult) LastInsertId() (int64, error) {
	return tr.LId, nil
}

func (tr *taosResult) RowsAffected() (int64, error) {
	return tr.RAf, nil
}
