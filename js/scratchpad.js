(function() {
  'use strict';
  $(document).ready(() => {
    let md = markdownit({
      linkify: true,
      typographer: true,
      quotes: '«»„“',
    });
    $('#markdown-it').html(md.render($('#markup').text()));
    $('#markup')
      .on('change keyup paste', e => {
        $('#markdown-it').html(md.render(e.target.value));
      })
      .on('keydown', e => {
        if (e.ctrlKey && e.which == 13) {
          $('#button-save').click();
        }
      });
  });
})();
// vim: ts=2 sts=2 sw=2 et
