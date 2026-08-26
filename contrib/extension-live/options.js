var field = document.getElementById('url');
var saved = document.getElementById('saved');

chrome.storage.local.get({ url: '' }, function (local) {
  if (local.url) {
    field.value = local.url;
    return;
  }
  chrome.storage.sync.get({ url: '' }, function (synced) { field.value = synced.url; });
});

field.addEventListener('input', function () {
  var url = field.value.trim();
  // Anything but http(s) would be a redirect to somewhere a new tab has
  // no business going.
  if (url !== '' && !/^https?:\/\//i.test(url)) {
    saved.textContent = 'needs to start with http:// or https://';
    return;
  }
  // Both stores: local is what a new tab reads, sync is what follows you
  // to another machine.
  chrome.storage.local.set({ url: url });
  chrome.storage.sync.set({ url: url }, function () {
    saved.textContent = url === '' ? 'cleared' : 'saved';
  });
});
