/**
 * Attempt to parse the given curl string.
 */

function parse_curl(s) {
    if (0 != s.indexOf('curl ')) return
    var args = rewrite(split(s))
    var out = { method: 'GET', header: {} }
    var state = ''

    args.forEach(function(arg){
        switch (true) {
            case isURL(arg):
                out.url = arg
                break;

            case arg == '-A' || arg == '--user-agent':
                state = 'user-agent'
                break;

            case arg == '-H' || arg == '--header':
                state = 'header'
                break;

            case arg == '-d' || arg == '--data' || arg == '--data-ascii' || arg == '--data-raw':
                state = 'data'
                break;

            case arg == '-u' || arg == '--user':
                state = 'user'
                break;

            case arg == '-I' || arg == '--head':
                out.method = 'HEAD'
                break;

            case arg == '-X' || arg == '--request':
                state = 'method'
                break;

            case arg == '-b' || arg =='--cookie':
                state = 'cookie'
                break;

            case arg == '--compressed':
                out.header['Accept-Encoding'] = out.header['Accept-Encoding'] || 'deflate, gzip'
                break;

            case !!arg:
                switch (state) {
                    case 'header':
                        var field = parseField(arg)
                        out.header[field[0]] = field[1]
                        state = ''
                        break;
                    case 'user-agent':
                        out.header['User-Agent'] = arg
                        state = ''
                        break;
                    case 'data':
                        if (out.method == 'GET' || out.method == 'HEAD') out.method = 'POST'
                        out.header['Content-Type'] = out.header['Content-Type'] || 'application/x-www-form-urlencoded'
                        out.body = out.body
                            ? out.body + '&' + arg
                            : arg
                        state = ''
                        break;
                    case 'user':
                        out.header['Authorization'] = 'Basic ' + btoa(arg)
                        state = ''
                        break;
                    case 'method':
                        out.method = arg
                        state = ''
                        break;
                    case 'cookie':
                        out.header['Set-Cookie'] = arg
                        state = ''
                        break;
                }
                break;
        }
    })

    return out
}

/**
 * Rewrite args for special cases such as -XPUT.
 */

function rewrite(args) {
    return args.reduce(function(args, a){
        if (0 == a.indexOf('-X')) {
            args.push('-X')
            args.push(a.slice(2))
        } else {
            args.push(a)
        }

        return args
    }, [])
}

/**
 * Parse header field.
 */

function parseField(s) {
    return s.split(/: (.+)/)
}

/**
 * Check if `s` looks like a url.
 */

function isURL(s) {
    // TODO: others at some point
    return /^https?:\/\//.test(s)
}


function scan(string, pattern, callback) {
    let result = "";

    while (string.length > 0) {
        const match = string.match(pattern);

        if (match && match.index != null && match[0] != null) {
            result += string.slice(0, match.index);
            result += callback(match);
            string = string.slice(match.index + match[0].length);
        } else {
            result += string;
            string = "";
        }
    }

    return result;
};

/**
 * Splits a string into an array of tokens in the same way the UNIX Bourne shell does.
 *
 * @param line A string to split.
 * @returns An array of the split tokens.
 */
function split(line = "") {
    const words = [];
    let field = "";
    scan(
        line,
        /\s*(?:([^\s\\'"]+)|'((?:[^'\\]|\\.)*)'|"((?:[^"\\]|\\.)*)"|(\\.?)|(\S))(\s|$)?/,
        (match) => {
            const [_raw, word, sq, dq, escape, garbage, separator] = match;

            if (garbage != null) {
                throw new Error(`Unmatched quote: ${line}`);
            }

            if (word) {
                field += word;
            } else {
                let addition;

                if (sq) {
                    addition = sq;
                } else if (dq) {
                    addition = dq;
                } else if (escape) {
                    addition = escape;
                }

                if (addition) {
                    field += addition.replace(/\\(?=.)/, "");
                }
            }

            if (separator != null) {
                words.push(field);
                field = "";
            }
        }
    );

    if (field) {
        words.push(field);
    }

    return words;
}

/**
 * Escapes a string so that it can be safely used in a Bourne shell command line.
 *
 * @param str A string to escape.
 * @returns The escaped string.
 */
function escape(str) {
    return str
        .replace(/([^A-Za-z0-9_\-.,:/@\n])/g, "\\$1")
        .replace(/\n/g, "'\n'");
}