// Run against rank.js by rank_test.go. The rows are the shape the page
// builds: a lowercased name, the host, and the aliases from the config.
// Most of them share one domain, because that is the page this filter
// exists for and the case the substring filter before it failed.
var rows = [
  { key: 'proxmox', host: '198.51.100.10', alias: ['pve', 'гипервизор'] },
  { key: 'nginx proxy manager', host: 'nginx.cdn.example.com', alias: ['npm', 'proxy'] },
  { key: 'forgejo', host: 'git.cdn.example.com', alias: ['git', 'гит', 'source control'] },
  { key: 'github', host: 'github.example', alias: ['гитхаб'] },
  { key: 'lookout', host: 'lookout.cdn.example.com', alias: ['monitoring', 'мониторинг'] },
  { key: 'vaultwarden', host: 'vault.cdn.example.com', alias: ['passwords', 'пароли'] },
  { key: 'youtube', host: 'youtube.example', alias: [] },
  { key: 'twitch', host: 'twitch.example', alias: [] },
  { key: 'коты коломны', host: 'cats.example', alias: [] },
  { key: 'immich', host: 'photo.cdn.example.com', alias: ['фото'] },
  { key: 'artificial analysis', host: 'aa.example', alias: [] }
];
var NAME_PREFIX = 0, ALIAS_PREFIX = 1, WORD = 2, ALIAS_WORD = 3,
    INITIALS = 4, HOST = 5, INSIDE = 6, ALIAS_INSIDE = 7, HOST_INSIDE = 8;

// What the page does with the scores: drop the misses, best first, ties
// broken by the order the rows are written in.
function order(t) {
  var out = [];
  for (var i = 0; i < rows.length; i++) {
    var r = newtabRank.rank(rows[i], t);
    if (r !== newtabRank.NONE) { out.push({ k: rows[i].key, r: r, i: i }); }
  }
  out.sort(function (x, y) { return x.r - y.r || x.i - y.i; });
  return out.map(function (o) { return o.k; });
}

var fail = 0;
function say(m) { fail++; console.log(m); }
function best(t, want) {
  var got = order(t)[0] || '';
  if (got !== want) { say('best(' + JSON.stringify(t) + ') = ' + JSON.stringify(got) + ', want ' + JSON.stringify(want)); }
}
function tier(row, t, want) {
  var got = newtabRank.tier(rows[row], t);
  if (got !== want) { say('tier(' + rows[row].key + ', ' + JSON.stringify(t) + ') = ' + got + ', want ' + want); }
}
function misses(row, t) {
  if (newtabRank.tier(rows[row], t) !== newtabRank.NONE) { say(rows[row].key + ' matched ' + JSON.stringify(t) + ' and should not have'); }
}

// One letter is the start of a name, not a letter found anywhere in one.
best('y', 'youtube');
best('l', 'lookout');
best('t', 'twitch');
misses(1, 'x');       // the x in nginx
misses(5, 'w');       // the w in vaultwarden
// Two letters, each of which the substring filter handed to whichever row
// came first — usually the one whose host it had matched.
best('lo', 'lookout');
best('gi', 'github');
best('tw', 'twitch');
best('va', 'vaultwarden');
// A letter inside a word still matches. It just loses, which is the fix.
tier(7, 'tw', NAME_PREFIX);
tier(5, 'tw', INSIDE);
// Every field is scored, and scored differently.
tier(1, 'npm', ALIAS_PREFIX);
tier(1, 'manager', WORD);
tier(2, 'control', ALIAS_WORD);
tier(5, 'words', ALIAS_INSIDE);
tier(2, 'fo', NAME_PREFIX);
tier(4, 'ito', ALIAS_INSIDE);
// Initials nobody writes down, and the name a row answers to on the LAN.
// "npm" is an alias of the Nginx row, so the initials tier is exercised
// on a row that has no alias to answer first.
tier(10, 'aa', INITIALS);
// Where the thresholds sit, each of them stated once: one character is
// enough for the start of a name, an alias or a word in either; two is
// the least that reaches a host or the middle of a word; three is the
// least that reaches the middle of a host.
tier(4, 'm', ALIAS_PREFIX);        // monitoring
tier(1, 'm', WORD);                // manager, in nginx proxy manager
tier(2, 'c', ALIAS_WORD);          // control, in the alias source control
tier(9, 'ph', HOST);
misses(9, 'p');
tier(9, 'hot', HOST_INSIDE);
misses(9, 'ho');
tier(9, 'photo', HOST);
tier(4, 'lookou', NAME_PREFIX);
// The domain and the label every row shares decide nothing: the first
// label is all of a host that is looked at, and never below two letters.
misses(4, 'cd');
misses(4, 'cdn');
misses(4, 'example');
misses(4, 'com');
tier(4, 'loo', NAME_PREFIX);
// Cyrillic is a name and an alias like any other.
best('мон', 'lookout');
best('ко', 'коты коломны');
tier(8, 'коломны', WORD);
tier(2, 'гит', ALIAS_PREFIX);
best('гитх', 'github');
// Several words are several conditions: all of them have to land.
best('proxy man', 'nginx proxy manager');
misses(2, 'hub');
if (newtabRank.rank(rows[2], 'git hub') !== newtabRank.NONE) { say('a missing word did not disqualify a row'); }
// Between two rows whose worst word is the same, the one that matched
// the other word better wins — worst word first, sum after it.
if (!(newtabRank.rank({ key: 'alpha beta', host: 'x.example', alias: [] }, 'alpha bet') <
      newtabRank.rank({ key: 'gamma alpha', host: 'x.example', alias: [] }, 'lpha bet'))) {
  say('the sum did not break a tie between equal worst words');
}
// A miss stays a miss however many words it is made of.
if (newtabRank.rank(rows[0], 'zz') !== newtabRank.NONE) { say('nonsense matched'); }
// However long the query, a worse worst word loses. A fixed multiplier
// would not hold: with enough words the sum of the better row reaches
// into the tier of the worse one. 144 of them is where 1000 broke.
var scraped = { key: 'zzz', host: 'zzz.example', alias: ['xximmich', 'qqphotoqq'] };
var named = { key: 'immich', host: 'yyphotoyy.example', alias: [] };
var words = ['photo'];
for (var i = 0; i < 143; i++) { words.push('immich'); }
var long = words.join(' ');
// Every word of it lands on the alias of one row, in the middle; on the
// other it is the name itself, except the one word that is only in the
// host. The row that never did worse than a mid-alias hit wins.
if (!(newtabRank.rank(scraped, long) < newtabRank.rank(named, long))) {
  say('a long query let a sum outweigh the worst word');
}

console.log(fail === 0 ? 'ok' : 'fail');
