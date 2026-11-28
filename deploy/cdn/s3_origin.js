// Local CDN stand-in (LOCAL_DEVELOPMENT §3.1, ADR-0044 §2): AWS Signature V4 of the origin request that nginx sends to
// floci for ew-media-private, after secure_link has accepted the signed CDN URL. floci runs with ENFORCE_AUTH and the
// private bucket keeps its public access block, so only a signed origin request reads it (the production CDN reads its
// private origin with its own credential the same way). Local/CI only: the credential comes from the environment of
// the cdn service (docker-compose.yml), never from this file.
//
// Signed: GET/HEAD, the object path ($s3_path set by each location: $uri, or for an HLS directory token the path without
// its /t/<md5>/<expires> prefix — the location regexes allow [A-Za-z0-9._/-] only, so it is already its S3 canonical
// form), no query string (proxy_pass sends the object path without the md5/expires of the CDN URL), headers host,
// x-amz-content-sha256 (UNSIGNED-PAYLOAD) and x-amz-date.
const crypto = require('crypto');

function hmac(key, data) {
  return crypto.createHmac('sha256', key).update(data).digest();
}

// amzDate is $s3_date: the request time as YYYYMMDD'T'HHMMSS'Z'. js_set values are cached for the request, so the
// x-amz-date header and the signature always use the same value.
function amzDate(r) {
  return new Date().toISOString().replace(/[-:]/g, '').replace(/\.\d{3}/, '');
}

// authorization is $s3_auth: the Authorization header of the origin request.
function authorization(r) {
  const v = r.variables;
  const date = v.s3_date;
  const day = date.slice(0, 8);
  const payload = 'UNSIGNED-PAYLOAD';
  const signedHeaders = 'host;x-amz-content-sha256;x-amz-date';
  const canonicalHeaders = 'host:' + v.s3_host + '\n' + 'x-amz-content-sha256:' + payload + '\n' + 'x-amz-date:' + date + '\n';
  const method = r.method === 'HEAD' ? 'HEAD' : 'GET';
  const canonical = [method, v.s3_path || v.uri, '', canonicalHeaders, signedHeaders, payload].join('\n');
  const scope = day + '/' + v.s3_region + '/s3/aws4_request';
  const toSign = ['AWS4-HMAC-SHA256', date, scope, crypto.createHash('sha256').update(canonical).digest('hex')].join('\n');
  const key = hmac(hmac(hmac(hmac('AWS4' + v.s3_secret_key, day), v.s3_region), 's3'), 'aws4_request');
  const signature = crypto.createHmac('sha256', key).update(toSign).digest('hex');
  return 'AWS4-HMAC-SHA256 Credential=' + v.s3_access_key + '/' + scope + ', SignedHeaders=' + signedHeaders + ', Signature=' + signature;
}

export default { amzDate, authorization };
