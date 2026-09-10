// How a query picks a row. It is a separate file from the rest of the
// client because it is the one part of the page with no DOM in it: given
// {key, host, alias} and a query it returns a number, which is what makes
// it something a test can hold still.
//
// A row used to win by being the first in the config whose name, host and
// aliases — joined into one string — contained the query anywhere. On a
// page whose rows all sit under one domain that is not a search: two
// letters of that domain matched every row, so the answer was whichever
// row came first, and a single letter matched whatever happened to
// contain it. Matches are ranked instead, and a short query is held to
// the front of a word, which is where an abbreviation actually lands.
var newtabRank = (function () {
  // No match, and a number every real score compares below.
  var NONE = Infinity;
  // The worst tier a match can have. It bounds the sum of a query's
  // words, which is what keeps the tie-break below the tier it breaks.
  var WORST = 8;

  function starts(s, t) { return s.lastIndexOf(t, 0) === 0; }

  function wordStart(s, t) {
    for (var i = 0; i < s.length; i++) {
      if (i === 0 || ' -.'.indexOf(s.charAt(i - 1)) !== -1) {
        if (s.lastIndexOf(t, i) === i) { return true; }
      }
    }
    return false;
  }

  // "npm" for Nginx Proxy Manager: the abbreviation nobody writes down.
  function initials(s) {
    var w = s.split(/[ .-]+/), out = '';
    for (var i = 0; i < w.length; i++) { if (w[i]) { out += w[i].charAt(0); } }
    return out;
  }

  // Lower is better, and the order of the tiers is the whole design.
  // A single character is the start of a name, of an alias or of a word
  // in either, and nothing else: one letter found in the middle of a
  // word is not evidence that anybody meant that row. From two
  // characters up the weaker kinds of match count as well — initials,
  // the host, and a letter sequence inside a word — in that order.
  //
  // Only the first label of a host is looked at, and never below two
  // characters. The rest of a host is shared: on a page where forty rows
  // sit under one domain, matching the domain is matching nothing, which
  // is how the filter this replaced came to answer "lo" with whatever
  // row happened to come first.
  function tier(row, t) {
    var name = row.key || '', alias = row.alias || [], i;
    var host = (row.host || '').split('.')[0];
    if (t === '') { return 0; }
    if (starts(name, t)) { return 0; }
    for (i = 0; i < alias.length; i++) { if (starts(alias[i], t)) { return 1; } }
    if (wordStart(name, t)) { return 2; }
    for (i = 0; i < alias.length; i++) { if (wordStart(alias[i], t)) { return 3; } }
    if (t.length < 2) { return NONE; }
    if (starts(initials(name), t)) { return 4; }
    // A row is often reached for by the name it answers to on the LAN —
    // photo, vault, git — which is its first label and nothing more.
    if (starts(host, t)) { return 5; }
    if (name.indexOf(t) !== -1) { return 6; }
    for (i = 0; i < alias.length; i++) { if (alias[i].indexOf(t) !== -1) { return 7; } }
    if (t.length >= 3 && host.indexOf(t) !== -1) { return WORST; }
    return NONE;
  }

  // Several words are several conditions: every one of them has to land
  // somewhere on the row, and the row is only as good as its worst word.
  // The sum breaks a tie between two rows whose worst word is the same:
  // a name matched exactly and a host scraped is a better answer than
  // two hosts scraped. What comes out is a comparable number rather than
  // a tier, and the multiplier is derived from the query so that no
  // number of words can let a sum reach into the tier above — every row
  // is scored against the same query, so the scale is the same for all
  // of them.
  function rank(row, t) {
    var parts = t.split(/\s+/), worst = 0, sum = 0;
    for (var i = 0; i < parts.length; i++) {
      var r = tier(row, parts[i]);
      if (r === NONE) { return NONE; }
      if (r > worst) { worst = r; }
      sum += r;
    }
    return worst * (WORST * parts.length + 1) + sum;
  }

  // tier is exported for the test: rank answers which row wins, tier
  // answers why, and a test that can only see the winner passes on a
  // ranking that is right for the wrong reason.
  return { rank: rank, tier: tier, NONE: NONE };
})();
