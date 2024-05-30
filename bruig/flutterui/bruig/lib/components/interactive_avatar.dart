import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/gc_context_menu.dart';
import 'package:bruig/components/user_context_menu.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class InteractiveAvatar extends StatelessWidget {
  const InteractiveAvatar({
    super.key,
    required this.chatNick,
    this.onTap,
    this.onSecondaryTap,
    this.avatar,
    this.radius,
  });

  final String chatNick;
  final VoidCallback? onTap;
  final VoidCallback? onSecondaryTap;
  final ImageProvider? avatar;
  final double? radius;

  @override
  Widget build(BuildContext context) {
    var nickInitial = chatNick.isNotEmpty ? chatNick[0].toUpperCase() : "?";
    return Consumer<ThemeNotifier>(builder: (context, themeNtf, _) {
      var theme = themeNtf.getTheme();
      var avatarColor = colorFromNick(chatNick, theme.brightness);
      var darkAvatarTextColor = theme.primaryColorDark;
      var lightAvatarTextColor = theme.primaryColorLight;
      var avatarTextColor =
          ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
              ? darkAvatarTextColor
              : lightAvatarTextColor;
      var selectedBackgroundColor = theme.highlightColor;

      return Material(
        color: selectedBackgroundColor.withOpacity(0),
        child: MouseRegion(
          cursor: SystemMouseCursors.click,
          child: GestureDetector(
            onTap: onTap,
            onSecondaryTap: onSecondaryTap,
            child: CircleAvatar(
                radius: radius,
                backgroundColor: avatarColor,
                backgroundImage: avatar,
                child: avatar != null
                    ? const Empty()
                    : Text(nickInitial,
                        style: TextStyle(
                            color: avatarTextColor,
                            fontSize: themeNtf.getLargeFont(context)))),
          ),
        ),
      );
    });
  }
}

class AvatarModelAvatar extends StatefulWidget {
  final AvatarModel avatar;
  final String nick;
  final VoidCallback? onTap;
  final VoidCallback? onSecondaryTap;
  final double? radius;

  const AvatarModelAvatar(
    this.avatar,
    this.nick, {
    this.onTap,
    this.onSecondaryTap,
    this.radius,
    super.key,
  });

  @override
  State<AvatarModelAvatar> createState() => _AvatarModelAvatarState();
}

class _AvatarModelAvatarState extends State<AvatarModelAvatar> {
  ImageProvider? avatarImg;

  void updateAvatarImg() {
    setState(() => avatarImg = widget.avatar.image);
  }

  @override
  void initState() {
    super.initState();
    widget.avatar.addListener(updateAvatarImg);
    updateAvatarImg();
  }

  @override
  void didUpdateWidget(AvatarModelAvatar oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.avatar != widget.avatar) {
      oldWidget.avatar.removeListener(updateAvatarImg);
      widget.avatar.addListener(updateAvatarImg);
      updateAvatarImg();
    }
  }

  @override
  void dispose() {
    widget.avatar.removeListener(updateAvatarImg);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return InteractiveAvatar(
      radius: widget.radius,
      chatNick: widget.nick,
      onTap: widget.onTap,
      onSecondaryTap: widget.onSecondaryTap,
      avatar: avatarImg,
    );
  }
}

// UserMenuAvatar displays the avatar of a chat and shows the context menu on tap.
class UserMenuAvatar extends StatelessWidget {
  final ClientModel client;
  final ChatModel chat;
  final VoidCallback? onTap;
  final VoidCallback? onSecondaryTap;
  final double? radius;
  final String? postFrom;

  const UserMenuAvatar(
    this.client,
    this.chat, {
    this.onTap,
    this.onSecondaryTap,
    this.radius,
    this.postFrom,
    super.key,
  });

  @override
  Widget build(BuildContext context) {
    return chat.isGC
        ? GcContexMenu(
            mobile: (context) => client.ui.chatSideMenuActive.chat = chat,
            targetGcChat: chat,
            child: AvatarModelAvatar(
              chat.avatar,
              chat.nick,
              radius: radius,
              onTap: onTap,
              onSecondaryTap: onSecondaryTap,
            ),
          )
        : UserContextMenu(
            client: client,
            targetUserChat: chat,
            targetUserId: chat.id,
            postFrom: postFrom,
            child: AvatarModelAvatar(
              chat.avatar,
              chat.nick,
              radius: radius,
              onTap: onTap,
              onSecondaryTap: onSecondaryTap,
            ),
          );
  }
}

// SelfAvatar displays the avatar of the local client.
class SelfAvatar extends StatelessWidget {
  final ClientModel client;
  final VoidCallback? onTap;
  const SelfAvatar(this.client, {this.onTap, super.key});

  @override
  Widget build(BuildContext context) {
    return AvatarModelAvatar(client.myAvatar, client.nick, onTap: onTap);
  }
}

// UserOrSelfAvatar displays the user avatar when chat != null or the local
// client avatar ("self") when chat == null.
class UserOrSelfAvatar extends StatelessWidget {
  final ClientModel client;
  final ChatModel? chat;
  final String? postFrom;
  const UserOrSelfAvatar(this.client, this.chat, {this.postFrom, super.key});

  @override
  Widget build(BuildContext context) {
    return chat != null
        ? UserMenuAvatar(client, chat!, postFrom: postFrom)
        : SelfAvatar(client);
  }
}

// UserAvatarFromID displays the avatar for the user ID. When that id is the local
// client id, it displays the local client avatar. If the id is unknown, displays
// a generic avatar.
class UserAvatarFromID extends StatelessWidget {
  final ClientModel client;
  final String uid;
  final bool disableTooltip;
  const UserAvatarFromID(this.client, this.uid,
      {this.disableTooltip = false, super.key});

  @override
  Widget build(BuildContext context) {
    if (uid == client.publicID) {
      return SelfAvatar(client);
    }

    var chat = client.getExistingChat(this.uid);
    if (chat != null) {
      return UserMenuAvatar(client, chat);
    }

    if (disableTooltip) {
      return InteractiveAvatar(chatNick: uid);
    }

    return Tooltip(
      message: "Unknown user $uid",
      child: InteractiveAvatar(chatNick: uid),
    );
  }
}
