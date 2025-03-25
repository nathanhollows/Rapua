---
title: "Roadmap"
sidebar: true
order: 6
---

# Roadmap and wishlist

If you want to [request a feature](https://github.com/nathanhollows/Rapua/issues/new?assignees=&labels=&projects=&template=feature_request.md) or want to check the progress of a feature, please check out the project on [GitHub](https://github.com/nathanhollows/Rapua/issues).

## New content blocks

I have the following content blocks in mind:

- **Photo challenge**: A block that allows users to take a photo and submit it.
- **Video challenge**: A block that allows users to record a video and submit it.
- **Sort list**: A block that allows users to sort a list of items.
- **Survey**: A block that allows users to answer a survey.
- **Quiz**: A block that allows users to answer a quiz.
- **API**: A block that only can only be completed by calling an API. This would enable facilitators to integrate with other systems, e.g., a student sends an email to a specific address, which triggers the API to mark the block as complete ([#41](https://github.com/nathanhollows/Rapua/issues/41)).

Updates to existing blocks:

- **Image Block**: Add support for image uploads ([#21](https://github.com/nathanhollows/Rapua/issues/21)).

## Admin tools to help users

Admins should be able to help players who are stuck. This could include marking a block as complete or fast-tracking a team by awarding points.

Track progress: https://github.com/nathanhollows/Rapua/issues/21

## User account settings

A lot of basic functionality is missing from the user account settings. There ought to be a dedicated page for users to update their email address, password, and other settings.

## Theming and Themes

I would quite like to have theme system. At first, it could offer pre-built themes that users can choose from. Additionally, it would be fairly easy to override the default theme with css variables.

![Theme demonstration from [DaisyUI](https://daisyui.com/)](/static/images/docs/developer/themes.png)

## Public game repository

I would like to have a public repository of games that users can play. Such a system would need to allow for time-based access to games, and would need to be able to track user progress. This was initially suggested in [this issue](https://github.com/nathanhollows/Rapua/issues/11).

## Team accounts

Rapua currently only supports anonymous users. I would like to add support for team accounts, where multiple users can be part of a team and share progress. This would also be useful for the public game repository.

## Chat system

A chat system would be useful for users to communicate with each other. This could be as simple as a chat room, or as complex as a direct messaging system.

Also included in this is making sure the notification/alerts system is robust and user-friendly.

## Reports for admin users

A feature that would be useful for admin users is the ability to generate reports. This could be as simple as a list of users and their progress, or as complex as a graph of user activity over time.

## Data dumps

I would like to add the ability to export user data. This could be useful for administrators who want to analyze user data in a spreadsheet.
