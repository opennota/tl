/* global Cookies, book_id, fragments_total:true, fragments_translated:true */
(function() {
  'use strict';
  let cancelEdit = null;
  let cancelEditOrig = null;
  let $previous = null;

  function updateProgress(total, translated) {
    if (total != fragments_total || translated != fragments_translated) {
      fragments_total = total;
      fragments_translated = translated;
      let pct = (total === 0) ? 0 : Math.floor(100 * translated / total);
      $('.progress-bar').attr('style', 'width:' + pct + '%');
      $('.progress .percent').text(pct);
      $('.progress .fraction').attr('title', translated + '/' + total);
    }
    $('.orig-empty-alert').toggle(!total);
  }

  function edit(e) {
    $previous = null;
    if (cancelEdit) cancelEdit();
    let $target = $(e.target);
    let $row = $target.closest('tr');
    let fid = $row.attr('id').substr(1);
    let $div = $target.closest('div[id^=v]');
    let vid = $div.length ? $div.attr('id').substr(1) : 0;
    let $form = $($('#translate-form-tmpl').html());
    let origLength = $row.find('td.o .text').text().replace(/\n/g, '').length;
    $form.find('.cnt-o').text(origLength);
    $form.attr('action', '/book/' + book_id + '/' + fid + '/translate');
    $form.find('[name=version_id]').attr('value', vid);
    let $submit = $form.find(':submit');
    $submit.text(vid ? 'Save' : 'Add');
    let $textarea = $form.find('textarea');
    let text = $div.find('.text').text();
    $textarea.text(text);
    $textarea.on('keyup change blur click', () => {
      $form.find('.cnt-t').text($textarea.val().replace(/\n/g, '').length);
    }).keyup();
    let $next = null;
    $form.ajaxForm({
      dataType: 'json',
      beforeSerialize: () => {
        let text = $textarea.val();
        text = text.replace(/(^|\s)- /g, '$1— ');
        text = text.replace(/(^|[\s([])"(\S)/g, '$1«$2');
        text = text.replace(/(\S)"([\s.,!?:;…)\]]|$)/g, '$1»$2');
        text = text.replace(/\.\.\./g, '…');
        $textarea.val(text);
      },
      beforeSubmit: () => $submit.attr('disabled', true),
      success: data => {
        cancelEdit = null;
        let $html = $($('#version-tmpl').html());
        $html.attr('id', 'v' + data.id);
        $html.find('.text').html(data.text);
        updateProgress(fragments_total, data.fragments_translated);
        $form.replaceWith($html);
        $previous = $html;
        if ($next) {
          $next.click();
        }
      },
      error: data => {
        $submit.attr('disabled', false);
        let $alert = $($('#alert-tmpl').html());
        $alert.append(data.responseText);
        $form.find('.alert-container').html($alert);
      }
    });
    cancelEdit = () => {
      cancelEdit = null;
      $form.replaceWith($div);
      $previous = $div;
    };
    $textarea.on('keydown', e => {
      if (e.ctrlKey && e.which == 13) {
        e.stopPropagation();
        $next = null;
        if (!e.shiftKey) {
          let $nextRow = $row.next().next();
          let $nextDiv = $nextRow.find('div[id^=v]:first-child');
          if ($nextDiv.length) {
            $next = $nextDiv.find('.x-edit');
          } else {
            $next = $nextRow.find('td > .x-translate');
          }
        }
        if (!vid && $textarea.val() === '') {
          cancelEdit();
          if ($next) {
            $next.click();
          }
        } else if ($textarea.val() != text) {
          $form.find(':submit').click();
        } else if ($next && $next.length) {
          $next.click();
        } else {
          cancelEdit();
        }
      } else if (e.which == 27) {
        cancelEdit();
      }
    });
    $form.on('click', '.cancel', cancelEdit);
    if (vid) {
      $div.replaceWith($form);
    } else {
      $row.find('td.t').append($form);
    }
    $textarea.autoGrow().focus();
  }

  function closeCommentary(e) {
    let $target = $(e.target);
    let $commentaryRow = $target.closest('tr');
    $commentaryRow.removeClass('shown').find('td').html('');
    let $comment = $commentaryRow.prev().find('.x-comment');
    $comment.removeClass('fa-times-circle');
    if ($comment.data('comment')) {
      $comment.addClass('fa-comment');
    } else {
      $comment.addClass('fa-comment-o');
    }
  }

  function comment(e) {
    let $target = $(e.target);
    let $row = $target.closest('tr');
    let $commentaryRow = $row.next();
    $commentaryRow.toggleClass('shown');
    if (!$commentaryRow.hasClass('shown')) {
      $target.removeClass('fa-times-circle');
      if ($target.data('comment')) {
        $target.addClass('fa-comment');
      } else {
        $target.addClass('fa-comment-o');
      }
      return;
    }
    $target.removeClass('fa-comment fa-comment-o').addClass('fa-times-circle');
    let fid = $row.attr('id').substr(1);
    let $form = $($('#commentary-form-tmpl').html());
    $form.attr('action', '/book/' + book_id + '/' + fid + '/comment');
    let $submit = $form.find('.btn-save');
    let $edit = $form.find('.btn-edit');
    let $div = $form.find('.text');
    let render = (text) => {
      let md = markdownit({
        linkify: true,
        typographer: true,
        quotes: '«»„“'
      });
      $div.html(md.render(String(text)));
      $div.on('dblclick', editCommentary);
      $submit.hide();
      $edit.show();
    };
    let editCommentary = () => {
      $div.off('dblclick');
      $edit.hide();
      let $textarea = $('<textarea name="text" spellcheck="false"></textarea>');
      $div.html('').append($textarea);
      $textarea.text($target.data('comment'));
      $submit.attr('disabled', false).show();
      $textarea.on('keydown', e => {
        if (e.ctrlKey && e.which == 13) {
          e.stopPropagation();
          $submit.click();
        }
      });
      // Delay autoGrow until the textarea's width property is initialized (Google Chrome).
      setTimeout(() => $textarea.autoGrow().focus(), 1);
    };
    $edit.on('click', editCommentary);
    $form.ajaxForm({
      dataType: 'json',
      beforeSubmit: () => $submit.attr('disabled', true),
      success: data => {
        $target.data('comment', data.text);
        render(data.text);
      },
      error: data => {
        $submit.attr('disabled', false);
        let $alert = $($('#alert-tmpl').html());
        $alert.append(data.responseText);
        $form.find('.alert-container').html($alert);
      }
    });
    let text = $target.data('comment');
    if (text) {
      render(text);
    } else {
      editCommentary();
    }
    $commentaryRow.find('td').html('').append($form);
  }

  function remove(e) {
    let $div = $(e.target).closest('div[id^=v]');
    let vid = $div.attr('id').substr(1);
    let fid = $div.closest('tr').attr('id').substr(1);
    let text = $div.find('p.text').html();
    let dlg = bootbox.confirm({
      message: '<b>Remove the following version?</b><br><br><blockquote>' + text + '</blockquote>',
      buttons: {
        confirm: {
          label: 'Remove',
          className: 'btn-danger'
        },
      },
      callback: result => {
        if (!result) return;
        $.ajax({
          method: 'DELETE',
          url: '/book/' + book_id + '/' + fid + '/' + vid
        }).done((data) => {
          dlg.modal('hide');
          $div.remove();
          updateProgress(fragments_total, data.fragments_translated);
        }).fail((xhr, status, err) => alert(err));
      }
    });
  }

  function star(e) {
    let $icon = $(e.target);
    let fid = $icon.closest('tr').attr('id').substr(1);
    $.ajax({
      method: 'POST',
      url: '/book/' + book_id + '/' + fid + '/star'
    }).done(() => {
      $icon.removeClass('x-star fa-star-o')
        .addClass('x-unstar fa-star');
    }).fail((xhr, status, err) => alert(err));
  }

  function unstar(e) {
    let $icon = $(e.target);
    let fid = $icon.closest('tr').attr('id').substr(1);
    $.ajax({
      method: 'DELETE',
      url: '/book/' + book_id + '/' + fid + '/star'
    }).done(() => {
      $icon.removeClass('x-unstar fa-star')
        .addClass('x-star fa-star-o');
    }).fail((xhr, status, err) => alert(err));
  }

  function toggleFluid() {
    let c = $('#container');
    if (c.hasClass('container-fluid')) {
      c.removeClass('container-fluid').addClass('container');
      Cookies.set('fluid', 0);
    } else {
      c.removeClass('container').addClass('container-fluid');
      Cookies.set('fluid', 1);
    }
  }

  function editOrig(e) {
    if (cancelEditOrig) cancelEditOrig();
    let $row = $(e.target).closest('tr');
    let fid = $row.attr('id').substr(1);
    let $div = $row.find('td.o > div');
    let $form = $($('#edit-orig-form-tmpl').html());
    $form.attr('action', '/book/' + book_id + '/' + fid);
    let $submit = $form.find(':submit');
    let $textarea = $form.find('textarea');
    $textarea.text($div.find('.text').text());
    $form.ajaxForm({
      dataType: 'json',
      beforeSubmit: () => $submit.attr('disabled', true),
      success: data => {
        cancelEditOrig = null;
        let $html = $($('#orig-tmpl').html());
        $html.find('.text').html(data.text);
        $form.replaceWith($html);
      },
      error: data => {
        $submit.attr('disabled', false);
        let $alert = $($('#alert-tmpl').html());
        $alert.append(data.responseText);
        $form.find('.alert-container').html($alert);
      }
    });
    cancelEditOrig = () => {
      cancelEditOrig = null;
      $form.replaceWith($div);
    };
    $textarea.on('keydown', e => {
      if (e.ctrlKey && e.which == 13) {
        $form.find(':submit').click();
      } else if (e.which == 27) {
        cancelEditOrig();
      }
    });
    $form.on('click', '.cancel', cancelEditOrig);
    $div.replaceWith($form);
    $textarea.autoGrow().focus();
  }

  function removeOrig(e) {
    let $row = $(e.target).closest('tr');
    let fid = $row.attr('id').substr(1);
    let text = $row.find('td.o p.text').html();
    let dlg = bootbox.confirm({
      message: '<b>Remove the following fragment?</b><br><br><blockquote>' + text + '</blockquote>',
      buttons: {
        confirm: {
          label: 'Remove',
          className: 'btn-danger'
        },
      },
      callback: result => {
        if (!result) return;
        $.ajax({
          method: 'DELETE',
          url: '/book/' + book_id + '/' + fid
        }).done((data) => {
          dlg.modal('hide');
          $row.remove();
          updateProgress(fragments_total - 1, data.fragments_translated);
          let num_filtered = +$('.button-filter sup').text();
          if (num_filtered) {
            $('.button-filter sup').text(num_filtered - 1);
          }
        }).fail((xhr, status, err) => alert(err));
      }
    });
  }

  function addOrig(e) {
    if (cancelEditOrig) cancelEditOrig();
    let $newRow = $($('#new-row-tmpl').html());
    let $textarea = $newRow.find('textarea');
    cancelEditOrig = () => {
      cancelEditOrig = null;
      $newRow.remove();
    };
    $newRow
      .on('click', '.cancel', cancelEditOrig)
      .on('click', '.x-orig-up', () => $newRow.prev().prev().before($newRow))
      .on('click', '.x-orig-down', () => $newRow.next().next().after($newRow));
    let $form = $newRow.find('form');
    $form.attr('action', '/book/' + book_id + '/fragments');
    let $submit = $form.find(':submit');
    $form.ajaxForm({
      dataType: 'json',
      beforeSerialize: ($form) => {
        let prev_id = $newRow.prev().prev().attr('id');
        let after = (prev_id) ? prev_id.substr(1) : '';
        $form.find('input[name=after]').attr('value', after);
      },
      beforeSubmit: () => $submit.attr('disabled', true),
      success: data => {
        cancelEditOrig = null;
        $newRow.find('td:first-child').html('<i class="fa fa-star-o x-star"></i>');
        $newRow.find('td:nth-child(3)').html('<i class="fa fa-arrow-right x-translate"></i>');
        $newRow.find('td:last-child').html('<i class="fa fa-comment-o x-comment"></i>');
        let $html = $($('#orig-tmpl').html());
        $html.find('.text').html(data.text);
        $html = $html.add('<a class="permalink" href="/book/' + book_id + '/' + data.id + '">#' + data.seq_num + '</a>');
        $newRow.find('td.o > form').replaceWith($html);
        $newRow.removeClass('editing').attr('id', 'f' + data.id);
        $newRow.after('<tr class="commentary"><td colspan="5"></td></tr>');
        updateProgress(fragments_total + 1, fragments_translated);
      },
      error: data => {
        $submit.attr('disabled', false);
        let $alert = $($('#alert-tmpl').html());
        $alert.append(data.responseText);
        $form.find('.alert-container').html($alert);
      }
    });
    if (e) {
      $(e.target).closest('tr').next().after($newRow);
    } else {
      $('.translator > tbody').append($newRow);
    }
    $textarea.autoGrow().focus();
  }

  function toggleOrigToolbox(e) {
    $('.translator').toggleClass('show-orig-toolbox');
    Cookies.set('show-orig-toolbox',
      $('.translator').hasClass('show-orig-toolbox') ? 1 : 0);
    $.scrollTo($(e.target).closest('tr'));
  }
  $(document).ready(() => {
    $('.translator')
      .on('click', '.x-translate, .x-edit', edit)
      .on('click', '.x-remove', remove)
      .on('click', '.x-comment', comment)
      .on('click', '.commentary-form .btn-close', closeCommentary)
      .on('click', '.x-star', star)
      .on('click', '.x-unstar', unstar)
      .on('click', '.x-expand', toggleOrigToolbox)
      .on('click', '.x-remove-orig', removeOrig)
      .on('click', '.x-edit-orig', editOrig)
      .on('click', '.x-add-orig', addOrig);
    $(document).on('keydown', e => {
      if (e.ctrlKey && e.which == 13) {
        if ($previous) {
          $previous.find('.x-edit').click();
        } else {
          $('td.t > div[id^=v]:first-child .x-edit').first().click();
        }
      }
    });
    $('.orig-empty-alert a').on('click', e => {
      e.preventDefault();
      addOrig();
    });
    updateProgress(fragments_total, fragments_translated);
    $('#filter-form')
      .on('click', 'label', e => {
        $(e.target).closest('label').find('[type="radio"]').prop('checked', true);
      })
      .on('click', '.dropdown-menu', e => e.stopPropagation());
    $('#orig_contains, #trans_contains').on('click', e => $(e.target).next().focus());
    $('.fa-window-restore').on('click', toggleFluid);
    if (location.hash) {
      $(location.hash).addClass('highlight');
    }
  });
})();
// vim: ts=2 sts=2 sw=2 et
