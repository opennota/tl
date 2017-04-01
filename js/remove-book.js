(function() {
  'use strict';
  $(document).ready(() => {
    let $btn = $('.button-remove');
    $(':radio').change(() => {
      $btn.attr('disabled', !$(':checked').length);
    });
    $btn.click(e => {
      e.preventDefault();
      let $checked = $(':checked');
      let bid = $checked.attr('value');
      let title = $checked.parent().text();
      let dlg = bootbox.confirm({
        message: '<b>Remove the following book?</b><br><br>' + title,
        buttons: {
          confirm: { label: 'Remove', className: 'btn-danger' },
        },
        callback: result => {
          if (!result) return;
          $.ajax({
            method: 'DELETE',
            url: '/book/' + bid,
          }).done(() => {
            dlg.modal('hide');
            location.href = '/';
          }).fail((xhr, status, err) => alert(err));
        }
      });
    });
  });
})();
