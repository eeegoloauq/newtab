// Redirect before anything is painted, so a new tab does not flash this
// page on the way to yours. replace(), not assign(): the empty tab has
// no business in the back history.
chrome.storage.sync.get({ url: '' }, function (cfg) {
  if (cfg.url) {
    location.replace(cfg.url);
    return;
  }
  var hint = document.getElementById('hint');
  hint.hidden = false;
  document.getElementById('open').addEventListener('click', function (e) {
    e.preventDefault();
    chrome.runtime.openOptionsPage();
  });
});
