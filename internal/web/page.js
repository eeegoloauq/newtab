// The whole client. It filters an already-rendered list and decides what
// Enter does. It never fetches anything: without it the page is still a
// complete set of links, which is why the markup is not built here.
(function () {
  var engine = document.body.dataset.engine;
  var said = document.getElementById('said');
  var main = document.querySelector('main');
  var q = document.getElementById('q');
  var links = Array.prototype.slice.call(document.querySelectorAll('a[data-key]'));
  var sections = Array.prototype.slice.call(document.querySelectorAll('[data-sec]'));
  var hit = null;

  // What gets hidden is the row, not the link inside it: hiding only the
  // anchor would leave the row's space behind.
  var items = links.map(function (a) { return a.parentNode; });

  function apply(term) {
    var t = term.trim().toLowerCase();
    if (hit) { hit.classList.remove('hit'); hit = null; }

    for (var i = 0; i < links.length; i++) {
      // No ranking: the config order is already the order
      // the operator thinks in, so the first match wins.
      var match = t === '' || links[i].dataset.key.indexOf(t) !== -1;
      items[i].classList.toggle('out', !match);
      if (match && !hit && t !== '') { hit = links[i]; }
    }
    // A section whose links are all filtered out would leave its heading
    // hanging over nothing.
    for (var j = 0; j < sections.length; j++) {
      sections[j].classList.toggle('out', !sections[j].querySelector('li:not(.out)'));
    }

    // The match Enter would open underlines itself. The same fact goes
    // to a screen reader, which cannot see an underline — out loud it is
    // the only feedback there is.
    if (hit) { hit.classList.add('hit'); }
    said.textContent = t === '' ? ''
      : hit ? document.body.dataset.opens + ': ' + hit.dataset.name
      : document.body.dataset.web;
  }

  // Filtering must not move the page under the reader: with the block
  // centred, a shorter list would slide the field down while they type,
  // and an empty result would drop it to the middle of the window. The
  // list keeps the height it had before anyone touched it.
  function reserve() {
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
    var live = links.filter(function (a, i) { return !items[i].classList.contains('out'); });
    if (live.length === 0) { return; }
    var at = live.indexOf(hit);
    var next = live[Math.min(Math.max(at + delta, 0), live.length - 1)] || live[0];
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
