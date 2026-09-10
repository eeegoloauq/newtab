// The whole client. It filters an already-rendered list and decides what
// Enter does. It never fetches anything: without it the page is still a
// complete set of links, which is why the markup is not built here.
(function () {
  var engine = document.body.dataset.engine;
  var said = document.getElementById('said');
  var main = document.querySelector('main');
  var q = document.getElementById('q');
  var hits = document.getElementById('hits');
  var links = Array.prototype.slice.call(document.querySelectorAll('a[data-key]'));
  // Columns are packed by height, so document order runs down one column
  // and then down the next. Config order is the order the operator thinks
  // in, and it is what breaks a tie between two equally good matches.
  links.sort(function (a, b) {
    return secOrder(a) - secOrder(b);
  });

  function secOrder(a) {
    return parseInt(sectionOf(a).style.order, 10) || 0;
  }

  function sectionOf(a) {
    return a.parentNode.parentNode.parentNode;
  }
  var hit = null;
  // The matches in the order they are shown, best first. Enter opens the
  // head of it and the arrow keys walk it.
  var shown = [];

  // What moves is the row, not the link inside it: moving only the anchor
  // would leave the row's space behind in the column it came from.
  var items = links.map(function (a) { return a.parentNode; });
  // The list each row belongs to when nothing is being filtered.
  var home = items.map(function (li) { return li.parentNode; });

  // Ranking lives in rank.js, which ships with this file. The DOM part
  // of it is only the row it is asked about.
  var NONE = newtabRank.NONE;

  function rowOf(a) {
    return {
      key: a.dataset.key,
      host: a.dataset.host || '',
      alias: a.dataset.alias ? a.dataset.alias.split('|') : []
    };
  }
  var rows = links.map(rowOf);

  function apply(term) {
    var t = term.trim().toLowerCase();
    if (hit) { hit.classList.remove('hit'); hit = null; }
    shown = [];

    var ranked = [], matched = [];
    for (var i = 0; i < links.length; i++) {
      var r = t === '' ? NONE : newtabRank.rank(rows[i], t);
      matched[i] = r !== NONE;
      if (matched[i]) { ranked.push({ a: links[i], r: r, i: i }); }
    }
    // Not every engine sorts stably, and the tie-break is the point:
    // between two matches of the same quality the config order wins.
    ranked.sort(function (x, y) { return x.r - y.r || x.i - y.i; });

    // The rows are moved into the results list rather than left where
    // they are and painted in a new order. CSS can reorder what the eye
    // sees and nothing else: tabbing and a screen reader would have gone
    // on reading the page in the order the columns happened to be packed
    // in, which is not the order it is now showing. The list is the
    // result, so it has to be the result in the document too.
    //
    // A row already standing in the right place is left alone: moving a
    // node blurs whatever inside it had the focus, and one more letter
    // typically leaves most of the list where it was.
    var active = document.activeElement;
    for (var k = 0; k < ranked.length; k++) {
      shown.push(ranked[k].a);
      var li = items[ranked[k].i];
      if (hits.childNodes[k] !== li) { hits.insertBefore(li, hits.childNodes[k] || null); }
    }
    // Everything else goes back to the section it came from, which is off
    // the screen while a query runs. An empty query sends every row home
    // rather than only the ones that are away: appending puts a row last
    // in its list, so a row that is already home still has to be put back
    // after the rows above it. In config order that restores the page
    // exactly as it was rendered.
    for (var n = 0; n < items.length; n++) {
      if (t === '' || (!matched[n] && items[n].parentNode !== home[n])) {
        home[n].appendChild(items[n]);
      }
    }

    document.body.classList.toggle('filtering', t !== '');

    // A row that was focused and has just left the screen takes the
    // focus with it into a hidden column. The field is where a reader
    // who is still typing expects to be anyway.
    if (t === '' && owed) { reserve(); }

    var at = active && active.dataset ? links.indexOf(active) : -1;
    if (at !== -1) {
      if (items[at].parentNode !== (t === '' ? home[at] : hits)) { q.focus(); }
      else if (document.activeElement !== active) { active.focus(); }
    }

    // The match Enter would open underlines itself. The same fact goes
    // to a screen reader, which cannot see an underline — out loud it is
    // the only feedback there is.
    if (t !== '' && shown.length) { hit = shown[0]; hit.classList.add('hit'); }
    said.textContent = t === '' ? ''
      : hit ? document.body.dataset.opens + ': ' + hit.dataset.name
      : document.body.dataset.web;
  }

  // Filtering must not move the page under the reader: with the block
  // centred, a shorter list would slide the field down while they type,
  // and an empty result would drop it to the middle of the window. The
  // list keeps the height it had before anyone touched it.
  // Set when a window resize arrived mid-query, so the measurement it
  // asked for can be taken once there is a whole page to measure again.
  var owed = false;

  function reserve() {
    // Never while a query is running: what would be measured then is the
    // handful of rows it left, and that height would become the floor
    // for the whole page once the field is cleared.
    if (document.body.classList.contains('filtering')) { owed = true; return; }
    owed = false;
    main.style.minHeight = '';
    main.style.minHeight = main.offsetHeight + 'px';
  }
  reserve();
  window.addEventListener('resize', reserve);

  q.addEventListener('input', function () { apply(q.value); });

  // Coming back from a search restores the page as it was left: the old
  // query still in the field and every link it did not match still
  // hidden. That is a correct restore and a useless page, so the filter
  // is cleared whenever the page is shown, restored from cache or not.
  window.addEventListener('pageshow', function () {
    if (q.value !== '') { q.value = ''; apply(''); }
  });

  // Arrow keys walk the matches. Without them the second match can only
  // be reached by typing more letters at it.
  function step(delta) {
    if (shown.length === 0) { return; }
    var at = shown.indexOf(hit);
    var next = shown[Math.min(Math.max(at + delta, 0), shown.length - 1)] || shown[0];
    if (hit) { hit.classList.remove('hit'); }
    hit = next;
    hit.classList.add('hit');
    said.textContent = document.body.dataset.opens + ': ' + hit.dataset.name;
  }

  document.getElementById('find').addEventListener('submit', function (e) {
    e.preventDefault();
    var term = q.value.trim();
    if (hit) { location.href = hit.href; return; }
    if (term === '') { return; }
    // Same tab on purpose: this page is the tab you opened to go
    // somewhere, so it is the one that should be replaced.
    location.href = engine.replace('%s', encodeURIComponent(term));
  });

  // Typing anywhere types into the field. Modified keys are left alone so
  // the browser's own shortcuts keep working.
  document.addEventListener('keydown', function (e) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      step(e.key === 'ArrowDown' ? 1 : -1);
      return;
    }
    // Escape clears from anywhere: after tabbing into the list the field
    // is no longer focused, and that is exactly when you want out.
    if (e.key === 'Escape') { q.value = ''; apply(''); q.focus(); return; }
    if (e.target === q) { return; }
    if (e.ctrlKey || e.metaKey || e.altKey) { return; }
    // Space scrolls the page. Every other printable key is the start of
    // a query.
    if (e.key.length === 1 && e.key !== ' ') { q.focus(); return; }
    if (e.key === 'Backspace') { e.preventDefault(); q.focus(); }
  });

  // A phone opens the keyboard on an autofocused field and loses half
  // the page to it. The attribute stays in the markup for the desktop.
  if (window.matchMedia && window.matchMedia('(pointer: coarse)').matches) { q.blur(); }
})();
