// Postman Pre-request Script: WebSocket HMAC Auth
// Signs the request as: GET|/ws|{timestamp}|{nonce}
// Requires collection variables: hmac_secret, user_id, user_type
// Sets collection variables: ws_timestamp, ws_nonce, ws_signature

const secret = pm.collectionVariables.get('hmac_secret');

if (!secret) {
    console.warn('HMAC secret is not set. Set collection variable "hmac_secret".');
} else {
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const nonceBytes = crypto.getRandomValues(new Uint8Array(16));
    const nonce = Array.from(nonceBytes).map(b => b.toString(16).padStart(2, '0')).join('');
    const path = '/ws';
    const payload = `GET|${path}|${timestamp}|${nonce}`;

    crypto.subtle.importKey(
        'raw',
        new TextEncoder().encode(secret),
        { name: 'HMAC', hash: 'SHA-256' },
        false,
        ['sign']
    ).then(key => crypto.subtle.sign('HMAC', key, new TextEncoder().encode(payload)))
    .then(signatureBuffer => {
        const signatureArray = new Uint8Array(signatureBuffer);
        const signature = Array.from(signatureArray).map(b => b.toString(16).padStart(2, '0')).join('');

        console.log('WS | Path:', path);
        console.log('WS | Timestamp:', timestamp);
        console.log('WS | Nonce:', nonce);
        console.log('WS | Payload:', payload);
        console.log('WS | Signature:', signature);

        pm.collectionVariables.set('ws_timestamp', timestamp);
        pm.collectionVariables.set('ws_nonce', nonce);
        pm.collectionVariables.set('ws_signature', signature);
    })
    .catch(err => console.error('WS | Crypto error:', err));
}
