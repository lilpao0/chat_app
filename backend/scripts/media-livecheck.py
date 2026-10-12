"""Manual live acceptance. Creates dedicated test users/messages; never prints tokens or capability URLs.

Requires installed Pillow and websocket-client. Run only against the explicitly selected deployment.
"""
import argparse
import datetime
import hashlib
import io
import json
import os
from pathlib import Path
import secrets
import urllib.error
import urllib.parse
import urllib.request
import uuid

from PIL import Image
import websocket


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--base-url', required=True)
parser.add_argument('--video', type=Path, required=True)
parser.add_argument('--report', type=Path, required=True)
parser.add_argument('--origin', default='http://localhost:5173')
parser.add_argument('--cleanup-session', type=Path, help='Optional private temporary file for cleanup cancellation verification')
args = parser.parse_args()
base = args.base_url.rstrip('/')
if urllib.parse.urlsplit(base).scheme != 'https':
    parser.error('Live API must use HTTPS')
opener = urllib.request.build_opener(NoRedirect)
report = {'timestamp': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'base_url': base, 'checks': [], 'fixtures': {}}
sockets = []


def check(name, ok):
    report['checks'].append({'name': name, 'pass': bool(ok)})
    if not ok:
        raise RuntimeError(name)


def raw(method, url, body=None, headers=None):
    req = urllib.request.Request(url, data=body, headers=headers or {}, method=method)
    try:
        with opener.open(req, timeout=45) as response:
            return response.status, response.headers, response.read(2 * 1024 * 1024)
    except urllib.error.HTTPError as error:
        return error.code, error.headers, error.read(2 * 1024 * 1024)


def api(name, token, method, path, payload=None, want=200):
    body = None if payload is None else json.dumps(payload).encode()
    headers = {'Content-Type': 'application/json', 'Origin': args.origin}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    status, response_headers, content = raw(method, base + path, body, headers)
    result = json.loads(content)
    if status != want:
        # Codes are safe diagnostics; never include request/response bodies or URLs.
        code = result.get('error', {}).get('code', 'unknown')
        raise RuntimeError(f'{name}: HTTP {status}, expected {want}; code={code}')
    check(name, True)
    check(name + ' API CORS', response_headers.get('Access-Control-Allow-Origin') == args.origin)
    if '/attachments' in path:
        check(name + ' no-store', response_headers.get('Cache-Control') == 'no-store')
    return result.get('data'), response_headers


def capability(url, upload=False):
    parsed = urllib.parse.urlsplit(url)
    check('Google HTTPS capability host', parsed.scheme == 'https' and parsed.netloc == 'storage.googleapis.com')
    if upload:
        check('Resumable capability', parsed.path.startswith('/upload/storage/v1/b/') and bool(urllib.parse.parse_qs(parsed.query).get('upload_id')))


def grant(token, conversation, content_type, size):
    result, _ = api('issue ' + content_type, token, 'POST', f'/api/conversations/{conversation}/attachments', {'content_type': content_type, 'size_bytes': size}, 201)
    capability(result['upload_url'], True)
    return result


def upload(result, blob, resume=False):
    url, content_type, total = result['upload_url'], result['content_type'], len(blob)
    headers = {'Content-Type': content_type, 'Origin': args.origin}
    start = 0
    if resume:
        start = 256 * 1024
        status, response_headers, _ = raw('PUT', url, blob[:start], dict(headers, **{'Content-Range': f'bytes 0-{start-1}/{total}'}))
        check('GCS first chunk 308', status == 308)
        check('GCS committed first chunk', response_headers.get('Range') == f'bytes=0-{start-1}')
        status, response_headers, _ = raw('PUT', url, b'', dict(headers, **{'Content-Range': f'bytes */{total}'}))
        check('GCS status query 308/Range', status == 308 and response_headers.get('Range') == f'bytes=0-{start-1}')
    status, response_headers, content = raw('PUT', url, blob[start:], dict(headers, **{'Content-Range': f'bytes {start}-{total-1}/{total}'}))
    check('GCS final chunk success', status in (200, 201))
    check('GCS upload browser origin', response_headers.get('Access-Control-Allow-Origin') == args.origin)
    metadata = json.loads(content)
    check('GCS actual size/type', int(metadata['size']) == total and metadata['contentType'] == content_type)
    return int(metadata['generation'])


def complete(token, attachment_id):
    return api('complete upload', token, 'POST', f'/api/attachments/{attachment_id}/complete', {})[0]


def access(token, attachment_id, blob, generation, video=False):
    result, _ = api('private access', token, 'GET', f'/api/attachments/{attachment_id}/access')
    capability(result['url'])
    query = urllib.parse.parse_qs(urllib.parse.urlsplit(result['url']).query)
    lifetime = int(query.get('X-Goog-Expires', ['0'])[0])
    check('Signed lifetime/generation', 899 <= lifetime <= 900 and query.get('generation') == [str(generation)])
    status, headers, content = raw('GET', result['url'], headers={'Origin': args.origin})
    check('Signed read exact bytes', status == 200 and hashlib.sha256(content).digest() == hashlib.sha256(blob).digest())
    check('Signed read browser CORS', headers.get('Access-Control-Allow-Origin') == args.origin)
    if video:
        status, headers, content = raw('GET', result['url'], headers={'Origin': args.origin, 'Range': 'bytes=0-99'})
        check('Video Range 206 bytes', status == 206 and content == blob[:100] and headers.get('Content-Range') == f'bytes 0-99/{len(blob)}')
    refreshed, _ = api('refresh access', token, 'GET', f'/api/attachments/{attachment_id}/access')
    check('Refreshed access stays pinned', urllib.parse.parse_qs(urllib.parse.urlsplit(refreshed['url']).query).get('generation') == [str(generation)])


def event(conn):
    return json.loads(conn.recv())


def main():
    run_id = uuid.uuid4().hex[:12]
    accounts = []
    for role in ('A', 'B', 'C'):
        email = f'media-release-{run_id}-{role.lower()}@example.test'
        password = secrets.token_urlsafe(24)
        user, _ = api('register fixture ' + role, None, 'POST', '/api/auth/register', {'first_name': 'Media test ' + role, 'email': email, 'password': password}, 201)
        login, _ = api('login fixture ' + role, None, 'POST', '/api/auth/login', {'email': email, 'password': password})
        accounts.append({'id': user['id'], 'email': email, 'token': login['access_token']})
    a, b, c = accounts
    report['fixtures']['accounts'] = [{'id': account['id'], 'email': account['email']} for account in accounts]
    report['fixtures']['owner_id'] = a['id']
    conversation, _ = api('open dedicated conversation', a['token'], 'POST', '/api/conversations/direct', {'user_id': b['id']}, 201)
    conversation_id = conversation['id']
    report['fixtures']['conversation_id'] = conversation_id
    prefix = f'/api/conversations/{conversation_id}'
    for account in (a, b):
        conn = websocket.create_connection(base.replace('https://', 'wss://', 1) + '/ws', header={'Authorization': 'Bearer ' + account['token']}, suppress_origin=True, timeout=20)
        sockets.append(conn)
    picture = Image.frombytes('RGB', (512, 512), os.urandom(512 * 512 * 3))
    output = io.BytesIO(); picture.save(output, format='PNG'); png = output.getvalue()
    output = io.BytesIO(); picture.resize((32, 32)).save(output, format='JPEG'); jpeg = output.getvalue()
    video = args.video.read_bytes()
    image_grant = grant(a['token'], conversation_id, 'image/png', len(png))
    api('incomplete completion', a['token'], 'POST', f"/api/attachments/{image_grant['id']}/complete", {}, 409)
    image_generation = upload(image_grant, png, resume=True)
    image_ready = complete(a['token'], image_grant['id'])
    check('Ready repeat completion unchanged', complete(a['token'], image_grant['id']) == image_ready)
    api('recipient cannot preview unsent', b['token'], 'GET', f"/api/attachments/{image_grant['id']}/access", want=404)
    api('recipient cannot complete owner upload', b['token'], 'POST', f"/api/attachments/{image_grant['id']}/complete", {}, 404)
    api('outsider cannot issue upload', c['token'], 'POST', prefix + '/attachments', {'content_type': 'image/png', 'size_bytes': len(png)}, 404)
    video_grant = grant(a['token'], conversation_id, 'video/mp4', len(video))
    video_generation = upload(video_grant, video)
    complete(a['token'], video_grant['id'])
    chats, _ = api('upload does not change chat', b['token'], 'GET', '/api/conversations')
    chat = next(item for item in chats if item['id'] == conversation_id)
    check('Upload creates no preview or unread', chat['last_message'] is None and chat['unread_count'] == 0)
    key = str(uuid.uuid4())
    ordered_ids = [video_grant['id'], image_grant['id']]
    payload = {'conversation_id': conversation_id, 'content': 'Live media acceptance', 'attachment_ids': ordered_ids}
    sockets[0].send(json.dumps({'type': 'send_message', 'request_id': key, 'data': payload}))
    events = [event(sockets[0]), event(sockets[0])]
    check('WS ack and event', {item['type'] for item in events} == {'message_sent', 'new_message'})
    saved = events[0]['data']
    check('WS ack/event same metadata', events[1]['data'] == saved)
    recipient = event(sockets[1])
    check('Recipient same metadata; no upload event', recipient['type'] == 'new_message' and recipient['data'] == saved)
    check('Original attachment order', [item['id'] for item in saved['attachments']] == ordered_ids)
    check('No capability in message', 'storage.googleapis.com' not in json.dumps(saved))
    retry_body = {'content': payload['content'], 'attachment_ids': ordered_ids, 'request_id': key}
    retry, _ = api('REST retry of WS send', a['token'], 'POST', prefix + '/messages', retry_body, 201)
    check('Durable retry same message', retry == saved)
    sockets[0].send(json.dumps({'type': 'send_message', 'request_id': key, 'data': payload}))
    replay = event(sockets[0]); check('WS retry only acknowledgement', replay['type'] == 'message_sent' and replay['data'] == saved)
    api('Retry reordered files conflicts', a['token'], 'POST', prefix + '/messages', dict(retry_body, attachment_ids=list(reversed(ordered_ids))), 409)
    access(b['token'], image_grant['id'], png, image_generation)
    access(b['token'], video_grant['id'], video, video_generation, video=True)
    api('Outsider cannot view attached', c['token'], 'GET', f"/api/attachments/{image_grant['id']}/access", want=404)
    attached = complete(a['token'], image_grant['id']); check('Attached complete does not reset', attached['state'] == 'attached' and attached['expires_at'] is None)
    text, _ = api('Text-only send', a['token'], 'POST', prefix + '/messages', {'content': 'Text still works', 'request_id': str(uuid.uuid4())}, 201)
    check('Text attachments array', text['attachments'] == [])
    for conn in sockets:
        check('Retry did not republish', event(conn)['data'] == text)
    single = grant(a['token'], conversation_id, 'image/jpeg', len(jpeg))
    single_generation = upload(single, jpeg); complete(a['token'], single['id'])
    media_only, _ = api('Image-only send', a['token'], 'POST', prefix + '/messages', {'attachment_ids': [single['id']], 'request_id': str(uuid.uuid4())}, 201)
    check('Media-only content', media_only['content'] == '' and len(media_only['attachments']) == 1)
    for conn in sockets:
        check('Media-only fanout', event(conn)['data'] == media_only)
    access(b['token'], single['id'], jpeg, single_generation)
    history, _ = api('Before history', b['token'], 'GET', prefix + f"/messages?before_id={text['id']}&limit=1")
    check('Before history same metadata', history == [saved])
    history, _ = api('After history', b['token'], 'GET', prefix + f"/messages?after_id={saved['id']}")
    check('After history ordered metadata', history == [text, media_only])
    chats, _ = api('Media preview/unread', b['token'], 'GET', '/api/conversations')
    chat = next(item for item in chats if item['id'] == conversation_id)
    check('Photo preview and unread per message', chat['last_message'] == 'Photo' and chat['unread_count'] == 3)
    api('Mark media read', b['token'], 'POST', prefix + '/read', {'last_read_message_id': media_only['id']})
    chats, _ = api('Read marker reflected', b['token'], 'GET', '/api/conversations')
    check('Media read clears unread', next(item for item in chats if item['id'] == conversation_id)['unread_count'] == 0)
    malformed = png[:64]
    failed = grant(a['token'], conversation_id, 'image/png', len(malformed))
    upload(failed, malformed)
    api('Malformed actual image fails', a['token'], 'POST', f"/api/attachments/{failed['id']}/complete", {}, 422)
    api('Permanent failure unusable', a['token'], 'POST', f"/api/attachments/{failed['id']}/complete", {}, 409)
    orphan = grant(a['token'], conversation_id, 'image/jpeg', len(jpeg))
    upload(orphan, jpeg)
    if args.cleanup_session:
        args.cleanup_session.write_text(json.dumps({'upload_url': orphan['upload_url'], 'size_bytes': len(jpeg), 'origin': args.origin}), encoding='utf-8')
    report['fixtures'].update(message_ids=[saved['id'], text['id'], media_only['id']], attached_ids=ordered_ids+[single['id']], failed_id=failed['id'], expire_id=orphan['id'])


try:
    main()
    report['status'] = 'PASS'
except Exception as error:
    report['status'] = 'FAIL'
    report['failure'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    for conn in sockets:
        conn.close()
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
    print(json.dumps({'status': report['status'], 'checks': len(report['checks']), 'failure': report.get('failure')}))
if report['status'] != 'PASS':
    raise SystemExit(1)
