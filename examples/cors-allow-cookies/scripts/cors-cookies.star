'''
This prep script makes the server usable across multiple predefined domains.
Note that this doesn't work with domains containing ports (e.g. localhost:8000)
'''

WHITELISTED_DOMAINS = [
    'http://domain1.com',
    'https://domain2.com',
    'https://sub.domain3.com',
]

ALLOWED_METHODS = 'POST, PUT, PATCH, GET, OPTIONS, DELETE'

# The main response editing function
def modify(req, resp):
    url = 'https://' if req['TLSEnabled'] == 'true' else 'http://'
    url += req.get('Header_Host', req['Host'])
    url += req['RequestURI']

    print('request: {url}. {size} bytes'.format(
        url=url, size=req['ContentLength']))
    print('resonse: {status}. {size} bytes'.format(
        status=resp['Status'], size=resp['ContentLength']))

    headers = {
        'X-Modified-By': 'cors-cookies',    # debug
        'proto': req['Proto']
    }

    if any([url.startswith(domain) for domain in WHITELISTED_DOMAINS]):
        headers['Access-Control-Allow-Credentials'] = 'true'
        headers['Access-Control-Allow-Origin'] = url
        preflight_header = req.get('Header_Access-Control-Request-Headers')
        if preflight_header:
            headers['Access-Control-Allow-Headers'] = preflight_header
        headers['Access-Control-Allow-Methods'] = ALLOWED_METHODS
        set_cookie = resp.get('Header_Set-Cookie')
        if set_cookie:
            val = set_cookie.split(';')
            headers['Set-Cookie'] = '{}; SameSite=None; Secure'.format(val[0])

    return {
        'headers': headers,
    }
