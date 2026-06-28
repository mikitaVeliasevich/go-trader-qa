package manager

import (
	"context"
	"fmt"
)

// FleetSubs fetches subaccounts and sync data for the given exchange account.
func (c *Client) FleetSubs(ctx context.Context, accountID int) (*SubsResponse, error) {
	path := fmt.Sprintf("/exchange/accounts/%d/subs", accountID)
	var resp SubsResponse
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("subs response ok=false for account %d", accountID)
	}
	return &resp, nil
}
