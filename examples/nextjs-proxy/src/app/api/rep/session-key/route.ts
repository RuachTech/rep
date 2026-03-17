// Dev-only session key endpoint for REP sensitive variable decryption.
// Returns 404 in production — the gateway serves this endpoint in prod.
export { GET } from '@rep-protocol/next/session-key';
