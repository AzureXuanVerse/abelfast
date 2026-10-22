package connection

// Kick tells the client it was disconnected (SC_10999, reasons in
// consts/disconnect_reasons.go) and closes its connection, between two of
// its packets so no handler is cut short. It fails without waiting when the
// client is closed or its queue is full.
func (client *Client) Kick(reason uint8) error {
	return client.Run(func(client *Client) error {
		if err := client.Disconnect(reason); err != nil {
			return err
		}
		err := client.flush()
		client.Close()
		return err
	})
}
