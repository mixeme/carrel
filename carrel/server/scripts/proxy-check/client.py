"""Drives Carrel behind nginx the way a browser and a DAV client would.

usage: client.py <tight|open>
Carrel sits behind nginx at http://nginx-pc (127.0.0.1:18090 from the host);
davloom is under /dav/. Radicale is reachable for setup at 127.0.0.1:15233.
"""
import base64, http.client, json, sys, urllib.parse

variant = sys.argv[1]
HOST = 'nginx-pc'
ADMIN, PASS = 'admin', 'Access-123456'
cookies = {}
failures = []


def req(method, path, body=None, headers=None, port=18090, auth=None, form=None):
    h = {'Host': HOST if port == 18090 else '127.0.0.1'}
    if cookies and port == 18090:
        h['Cookie'] = '; '.join(f'{k}={v}' for k, v in cookies.items())
    if form is not None:
        body = urllib.parse.urlencode(form)
        h['Content-Type'] = 'application/x-www-form-urlencoded'
    if auth:
        h['Authorization'] = 'Basic ' + base64.b64encode(f'{auth[0]}:{auth[1]}'.encode()).decode()
    h.update(headers or {})
    c = http.client.HTTPConnection('127.0.0.1', port, timeout=120)
    c.request(method, path, body=body, headers=h)
    r = c.getresponse()
    data = r.read().decode('utf-8', 'replace')
    for k, v in r.getheaders():
        if k.lower() == 'set-cookie':
            name, _, rest = v.partition('=')
            value = rest.split(';', 1)[0]
            if value:
                cookies[name] = value
            else:
                cookies.pop(name, None)
    return r.status, dict(r.getheaders()), data


def expect(what, ok):
    print(('ok   ' if ok else 'FAIL ') + what)
    if not ok:
        failures.append(what)


def csrf_row(page):
    for f in page.values():
        if isinstance(f, dict) and 'sets' in f:
            for s in f['sets'] or []:
                for row in s['rows'] or []:
                    if row['name'] == '_csrf':
                        return row['value']
    return ''


# First run, or sign in.
st, _, body = req('GET', '/setup')
if st == 200 and 'password2' in body:
    req('POST', '/setup', form={'login': ADMIN, 'password': PASS, 'password2': PASS, '_csrf': cookies.get('carrel_csrf', '')})
cookies.pop('carrel_sid', None)
req('GET', '/login')
st, hdr, _ = req('POST', '/login', form={'login': ADMIN, 'password': PASS, '_csrf': cookies.get('carrel_csrf', '')})
expect('sign-in through the proxy', st == 303 and 'carrel_sid' in cookies)
_, _, account = req('GET', '/app/settings/account')
token = csrf_row(json.loads(account))

# The installation check, at the address the proxy answers.
st, _, body = req('POST', '/admin/install', form={'url': 'http://nginx-pc', '_csrf': token})
page = json.loads(body)
rows = {r['check']: r for r in page.get('rows') or []}
for r in page.get('rows') or []:
    print(f"   {r['state']:8} {r['check']}: {r['value']}" + (f"  в†’ {r['advice']}" if r['advice'] else ''))
expect('eleven rows', len(rows) == 11)
for check in ['Reachable from outside', 'Base address', 'Scheme behind the proxy', 'Client address', 'Cookie', 'Clock', 'Data folder']:
    expect(f'{check} is ok behind nginx', rows.get(check, {}).get('state') == 'ok')
expect('Certificate is broken over plain http', rows.get('Certificate', {}).get('state') == 'bad')
expect('Long request timeout is grey', rows.get('Long request timeout', {}).get('state') == 'unknown')
want_upload = 'bad' if variant == 'tight' else 'ok'
expect(f'Upload size is {want_upload} with the {variant} proxy', rows.get('Upload size', {}).get('state') == want_upload)
expect('the event stream is not buffered (X-Accel-Buffering: no is honoured)', rows.get('Event stream (SSE)', {}).get('state') == 'ok')

if variant == 'open':
    # davloom under /dav/: the root page, then a device through the proxy.
    st, _, body = req('GET', '/dav/')
    expect('davloom root under /dav/', st == 200 and json.loads(body).get('title') == 'davloom')
    st, h, _ = req('PROPFIND', '/dav/principal/', headers={'Depth': '0'})
    expect('davloom asks for the device password', st == 401 and 'davloom' in h.get('Www-Authenticate', h.get('WWW-Authenticate', '')))

    mk = '<?xml version="1.0"?><C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Proxy check</D:displayname></D:prop></D:set></C:mkcalendar>'
    st, _, _ = req('MKCALENDAR', '/test/proxy-check/', body=mk, port=15233, auth=('test', 'test'), headers={'Content-Type': 'application/xml'})
    expect('a calendar on Radicale', st in (201, 405))
    st, _, body = req('POST', '/app/settings/connections', form={'server': 'http://radicale-pc:5232/test/', 'login': 'test', 'password': 'test', '_csrf': token})
    expect('Radicale connected through the guard', st == 303)
    req('POST', '/app/settings/unlock', form={'password': PASS, '_csrf': token}, headers={'Carrel-View': 'fragment', 'Carrel-Target': 'sheet'})
    _, _, body = req('GET', '/app/settings/davloom')
    dl = json.loads(body)
    form = {'op': 'issue', 'name': 'Proxy check', 'kind.event': 'on', 'kind.contact': 'on', '_csrf': token}
    for s in dl['add']['sets']:
        for row in s['rows']:
            if row['name'].startswith('col.'):
                form[row['name']] = 'on'
    st, _, _ = req('POST', '/app/settings/davloom', form=form)
    _, _, body = req('GET', '/app/settings/davloom')
    pw = ''
    for s in json.loads(body)['add']['sets']:
        for row in s['rows']:
            if row['name'] == 'password':
                pw = row['value']
    expect('a device password was issued', st == 303 and len(pw) > 40)
    st, _, cal = req('PROPFIND', '/dav/cal/', headers={'Depth': '1'}, auth=(ADMIN, pw))
    expect('the calendar home through nginx names /dav/c/…', st == 207 and '<d:href>/dav/c/' in cal)
    href = cal.split('<d:href>/dav/c/')[1].split('</d:href>')[0]
    col = '/dav/c/' + href
    ev = 'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//check//EN\r\nBEGIN:VEVENT\r\nUID:pc1\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20261001T100000Z\r\nSUMMARY:Behind nginx\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n'
    st, _, _ = req('PUT', col + 'pc1.ics', body=ev, auth=(ADMIN, pw), headers={'Content-Type': 'text/calendar', 'If-None-Match': '*'})
    expect(f'a PUT through nginx and davloom lands ({st})', st in (201, 204))
    st, _, listing = req('PROPFIND', col, headers={'Depth': '1'}, auth=(ADMIN, pw))
    ok = st == 207 and col + 'pc1.ics' in listing and '/test/proxy-check/' not in listing
    expect('the listing names /dav/c/.../pc1.ics and not the collection path on Radicale', ok)
    if not ok:
        print(listing[:3000])
    st, _, got = req('GET', col + 'pc1.ics', auth=(ADMIN, pw))
    expect('the event reads back through nginx', st == 200 and 'Behind nginx' in got)

print('failures:', len(failures))
sys.exit(1 if failures else 0)
