package routing

import "context"

const SyncMatrixLimit = 25

func BatchMatrix(ctx context.Context, provider Provider, req MatrixRequest, limit int) (MatrixResult, error) {
	if limit <= 0 {
		limit = SyncMatrixLimit
	}
	if len(req.Destinations) <= limit && len(req.Origins) <= limit {
		return provider.Matrix(ctx, req)
	}
	if len(req.Origins) > limit {
		return MatrixResult{}, ErrInvalidResponse
	}
	merged := MatrixResult{}
	var cells []MatrixCell
	for start := 0; start < len(req.Destinations); start += limit {
		end := start + limit
		if end > len(req.Destinations) {
			end = len(req.Destinations)
		}
		part := req
		part.Destinations = append([]Point(nil), req.Destinations[start:end]...)
		got, err := provider.Matrix(ctx, part)
		if err != nil {
			return MatrixResult{}, err
		}
		if start == 0 {
			merged = got
		}
		for _, cell := range got.Cells {
			cell.DestinationIndex += start
			cells = append(cells, cell)
		}
	}
	merged.Cells = cells
	return merged, nil
}
