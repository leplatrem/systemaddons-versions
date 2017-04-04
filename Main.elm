module Main exposing (..)

import Html
import Html.Attributes
import Html.Events


type alias Model =
    { email : String
    , submitted : Bool
    }


type Msg
    = EmailChange String
    | Submit


init : ( Model, Cmd Msg )
init =
    ( { email = "", submitted = False }, Cmd.none )


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none


view : Model -> Html.Html Msg
view model =
    case model.submitted of
        True ->
            Html.div []
                [ Html.text "Submitted "
                , Html.text model.email
                ]

        _ ->
            Html.label []
                [ Html.text "Email"
                , Html.input
                    [ Html.Attributes.name "email"
                    , Html.Attributes.placeholder "joe@bar"
                    , Html.Events.onInput EmailChange
                    ]
                    []
                , Html.button
                    [ Html.Events.onClick Submit
                    ]
                    [ Html.text "Submit"
                    ]
                ]


update : Msg -> Model -> ( Model, Cmd Msg )
update message model =
    case message of
        EmailChange email ->
            ( { model | email = email }, Cmd.none )

        Submit ->
            ( { model | submitted = True }, Cmd.none )


main =
    Html.program
        { init = init
        , subscriptions = subscriptions
        , view = view
        , update = update
        }
