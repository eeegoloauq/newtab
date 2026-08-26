// The whole client. It filters an already-rendered list and decides what
// Enter does. It never fetches anything: without it the page is still a
// complete set of links, which is why the markup is not built here.
(function () {
  var engine = document.body.dataset.engine;
  var q = document.getElementById('q');
  var links = Array.prototype.slice.call(document.querySelectorAll('a[data-key]'));
  var sections = Array.prototype.slice.call(document.querySelectorAll('[data-sec]'));
  var hit = null;

  // What gets hidden is the item, not the link: in a list that is the
  // <li> (hiding only the <a> would leave its bullet space behind), on a
  // card it is the card itself.
  var items = links.map(function (a) {
    return a.parentNode.tagName === 'LI' ? a.parentNode : a;
  });

  function apply(term) {
    var t = term.trim().toLowerCase();
    if (hit) { hit.classList.remove('hit'); hit = null; }

    for (var i = 0; i < links.length; i++) {
      // No ranking: 43 links, and the config order is already the order
      // the operator thinks in, so the first match wins.
      var match = t === '' || links[i].dataset.key.indexOf(t) !== -1;
      items[i].classList.toggle('out', !match);
      if (match && !hit && t !== '') { hit = links[i]; }
    }
    // A section whose links are all filtered out would leave its heading
    // hanging over nothing.
    for (var j = 0; j < sections.length; j++) {
      sections[j].classList.toggle('out', !sections[j].querySelector('li:not(.out), .card:not(.out)'));
    }

    // The match Enter would open underlines itself. A line of prose
    // explaining the keys was there and was removed: it said the same
    // thing on every one of the thousand times the page is opened.
    if (hit) { hit.classList.add('hit'); }
  }

  q.addEventListener('input', function () { apply(q.value); });

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
    if (e.target === q) {
      if (e.key === 'Escape') { q.value = ''; apply(''); }
      return;
    }
    if (e.ctrlKey || e.metaKey || e.altKey) { return; }
    if (e.key.length === 1) { q.focus(); return; }
    if (e.key === 'Backspace') { e.preventDefault(); q.focus(); }
  });
})();
