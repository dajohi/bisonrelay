import 'package:bruig/components/containers.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/emoji.dart';
import 'package:bruig/theme_manager.dart';
import 'package:emoji_picker_flutter/emoji_picker_flutter.dart';
import 'package:flutter/material.dart';

class TypingEmojiPanel extends StatefulWidget {
  final TypingEmojiSelModel model;
  const TypingEmojiPanel({required this.model, super.key});

  @override
  State<TypingEmojiPanel> createState() => _TypingEmojiPanelState();
}

class _TypingEmojiPanelState extends State<TypingEmojiPanel> {
  TypingEmojiSelModel get model => widget.model;
  List<Emoji> emojis = [];
  int selected = -1;
  ScrollController scroll = ScrollController();

  void updated() {
    setState(() {
      emojis = model.selectionList.toList();
    });
  }

  void updatedSelected() {
    setState(() {
      selected = model.selected.value;
      var offset = selected.toDouble() * 48; // 48 == height of tile
      if (scroll.hasClients && offset >= 0) {
        scroll.animateTo(offset,
            duration: const Duration(milliseconds: 500),
            curve: Easing.standard);
      }
    });
  }

  @override
  void initState() {
    super.initState();
    model.addListener(updated);
    model.selected.addListener(updatedSelected);
  }

  @override
  void dispose() {
    model.removeListener(updated);
    model.selected.removeListener(updatedSelected);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!model.isTypingEmoji) {
      return const Empty();
    }

    var theme = ThemeNotifier.of(context);

    return Box(
      color: SurfaceColor.primaryContainer,
      child: Container(
          constraints: const BoxConstraints(maxHeight: 200),
          child: Scrollbar(
            thumbVisibility: true,
            controller: scroll,
            child: ListView.builder(
              shrinkWrap: true,
              itemCount: emojis.length,
              controller: scroll,
              itemBuilder: (BuildContext context, int index) {
                var e = emojis[index];
                return ListTile(
                  onTap: () {},
                  // hoverColor: Colors.amber,
                  selectedTileColor: Colors.transparent,
                  selectedColor:
                      theme.extraColors.selectedItemOnSurfaceListView,
                  selected: index == selected,
                  leading: Txt.H(e.emoji),
                  title: Txt.M(e.name),
                );
              },
            ),
          )),
    );
  }
}
