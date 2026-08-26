// The address is kept in both stores: local answers immediately, sync
// carries it to your other machines. Reading local first is what keeps a
// new tab from showing anything at all before it redirects.
chrome.storage.local.get({ url: '' }, function (local) {
  if (local.url) {
    go(local.url);
    return;
  }
  chrome.storage.sync.get({ url: '' }, function (synced) {
    if (synced.url) {
      // Seen on this machine now; the next tab will not have to ask sync.
      chrome.storage.local.set({ url: synced.url });
      go(synced.url);
      return;
    }
    hint();
  });
});

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
