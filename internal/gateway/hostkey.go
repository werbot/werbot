package gateway

import gossh "golang.org/x/crypto/ssh"

// pinKey pins the target host key through the server, unless the snapshot
// already carries it as the pinned key of res. Any other key, also one the
// snapshot does not know yet, goes to the server, which decides: a key that
// differs from the pinned one is refused there. ClearHostKey changes the
// snapshot, so the next connection pins anew.
func (p *Pipeline) pinKey(res Resource, alg string, key []byte) error {
	if res.HostKeySHA256 != "" {
		if k, err := gossh.ParsePublicKey(key); err == nil && gossh.FingerprintSHA256(k) == res.HostKeySHA256 {
			return nil
		}
	}
	return p.ctl.PinHostKey(p.stop, res.ID, alg, key)
}
