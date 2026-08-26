// The address is kept in both stores: local answers immediately, sync
// carries it to your other machines. Reading local first is what keeps a
// new tab from showing anything at all before it redirects.
chrome.storage.local.get({ url: '', colour: '' }, function (local) {
  paint(local.colour);
  if (local.url) {
    go(local.url);
    return;
  }
  chrome.storage.sync.get({ url: '', colour: '' }, function (synced) {
    paint(synced.colour);
    if (synced.url) {
      // Seen on this machine now; the next tab will not have to ask sync.
      chrome.storage.local.set({ url: synced.url, colour: synced.colour || '' });
      go(synced.url);
      return;
    }
    hint();
  });
});

// Chrome paints this page for the moment between the keystroke and the
// page arriving. Left alone it is the browser's own colour, which is
// what the flash is; painted in the colour of the page you are going to,
// there is nothing to see. Other extensions solve it the same way.
// The stylesheet has already painted this dark; a colour here only makes
// the wait match the page more closely. Holding a copy of the page's
// wallpaper was tried and removed: it bought a fraction of a second and
// cost a permission prompt over every site you visit.
function paint(colour) {
  if (/^#[0-9a-f]{6}$/i.test(colour || '')) {
    document.documentElement.style.backgroundColor = colour;
  }
}

function go(url) {
  // replace(), not assign(): the empty tab has no business in the back
  // history.
  location.replace(url);
}

function hint() {
  var main = document.createElement('main');
  var first = document.createElement('p');
  first.textContent = 'No address set yet.';
  var second = document.createElement('p');
  var link = document.createElement('a');
  link.href = '#';
  link.textContent = 'Open the options';
  link.addEventListener('click', function (e) {
    e.preventDefault();
    chrome.runtime.openOptionsPage();
  });
  second.appendChild(link);
  second.appendChild(document.createTextNode(' and paste the address of your newtab.'));
  main.appendChild(first);
  main.appendChild(second);
  document.body.appendChild(main);
}
