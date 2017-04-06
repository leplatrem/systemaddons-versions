module View exposing (view)

import Dict
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (..)
import Model
    exposing
        ( Model
        , Release
        , SystemAddon
        , SystemAddonVersions
        , ReleaseDetails
        , Msg(..)
        , filterReleases
        )


joinBuiltinsUpdates : List SystemAddon -> List SystemAddon -> List SystemAddonVersions
joinBuiltinsUpdates builtins updates =
    builtinVersions builtins
        |> updateVersions updates
        |> List.sortBy .id


builtinVersions : List SystemAddon -> List SystemAddonVersions
builtinVersions builtins =
    List.map (\a -> SystemAddonVersions a.id (Just a.version) Nothing) builtins


updateVersions : List SystemAddon -> List SystemAddonVersions -> List SystemAddonVersions
updateVersions updates builtins =
    let
        mergeVersion : SystemAddonVersions -> SystemAddon -> SystemAddonVersions
        mergeVersion b u =
            SystemAddonVersions b.id b.builtin (Just u.version)

        hasBuiltin : List SystemAddonVersions -> SystemAddon -> Bool
        hasBuiltin addons addon =
            List.any (\i -> i.id == addon.id) addons

        addToBuiltin : List SystemAddonVersions -> SystemAddon -> List SystemAddonVersions
        addToBuiltin addons addon =
            List.map
                (\i ->
                    if i.id == addon.id then
                        mergeVersion i addon
                    else
                        i
                )
                addons
    in
        List.foldr
            (\a acc ->
                if hasBuiltin acc a then
                    addToBuiltin acc a
                else
                    List.append acc [ SystemAddonVersions a.id Nothing (Just a.version) ]
            )
            builtins
            updates


viewReleaseDetails : ReleaseDetails -> Html Msg
viewReleaseDetails details =
    table [ class "table table-condensed" ]
        [ thead []
            [ tr []
                [ th [] [ text "Build ID" ]
                , th [] [ text "Target" ]
                , th [] [ text "Lang" ]
                , th [] [ text "Channel" ]
                , th [] [ text "URL" ]
                ]
            ]
        , tbody []
            [ tr []
                [ td [] [ text details.buildId ]
                , td [] [ text details.target ]
                , td [] [ text details.lang ]
                , td [] [ text details.channel ]
                , td [] [ a [ href details.url, title details.url ] [ text "Link" ] ]
                ]
            ]
        ]


viewSystemAddonVersionsRow : SystemAddonVersions -> Html Msg
viewSystemAddonVersionsRow addon =
    tr []
        [ td [] [ text addon.id ]
        , td [] [ text <| Maybe.withDefault "" addon.builtin ]
        , td [] [ text <| Maybe.withDefault "" addon.update ]
        ]


viewSystemAddons : List SystemAddon -> List SystemAddon -> Html Msg
viewSystemAddons builtins updates =
    table [ class "table table-stripped table-condensed" ]
        [ thead []
            [ tr []
                [ th [] [ text "Id" ]
                , th [] [ text "Built-in" ]
                , th [] [ text "Updated" ]
                ]
            ]
        , tbody [] <|
            List.map viewSystemAddonVersionsRow <|
                joinBuiltinsUpdates builtins updates
        ]


viewRelease : Release -> Html Msg
viewRelease { details, builtins, updates } =
    div [ class "panel panel-default" ]
        [ div [ class "panel-heading" ] [ strong [] [ text details.filename ] ]
        , div [ class "panel-body" ]
            [ viewReleaseDetails details
            , h4 [] [ text "System Addons" ]
            , viewSystemAddons builtins updates
            ]
        ]


filterCheckbox : (String -> Bool -> Msg) -> ( String, Bool ) -> Html Msg
filterCheckbox handler ( name, active ) =
    li [ class "list-group-item" ]
        [ label []
            [ input
                [ type_ "checkbox"
                , value name
                , onCheck <| handler name
                , checked active
                ]
                []
            , text <| " " ++ name
            ]
        ]


channelFilters : Model -> Html Msg
channelFilters model =
    let
        channels =
            model.filters.channels
                |> Dict.toList
                |> List.map (filterCheckbox ToggleChannelFilter)
    in
        div [ class "panel panel-default" ]
            [ div [ class "panel-heading" ] [ strong [] [ text "Filter channels" ] ]
            , ul [ class "list-group" ] <| channels
            ]


view : Model -> Html Msg
view model =
    div [ class "container" ]
        [ div [ class "header" ]
            [ h1 [] [ Html.text "System Addons" ] ]
        , div [ class "row" ]
            [ div [ class "col-sm-9" ]
                [ div [] <| List.map viewRelease <| filterReleases model ]
            , div [ class "col-sm-3" ]
                [ channelFilters model ]
            ]
        ]
