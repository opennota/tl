(function() {
  'use strict';
  $.fn.editable.defaults.mode = 'inline';
  $.fn.editableform.loading = '';
  $.fn.editableform.buttons = $('#editable-buttons-tmpl').html();
  $(document).ready(() => {
    $('.title').each((_, el) => {
      let $title = $(el);
      let $edit = $title.next();
      $title.editable({
        type: 'text',
        toggle: 'manual',
        url: $title.attr('href'),
        params: params => ({ title: params.value }),
        send: 'always'
      })
      .on('hidden', () => $edit.show())
        .on('shown', () => $edit.hide());
      $edit.click(e => {
        e.stopPropagation();
        $title.editable('toggle');
      });
    });
  });
})();
