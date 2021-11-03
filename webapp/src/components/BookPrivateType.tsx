//System
import React from "react";
import { useHistory } from "react-router-dom";
import styled from "@emotion/styled";
import { useSelector } from "react-redux";

//MaterialUI
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import TextField from "@material-ui/core/TextField";
import HouseIcon from "@material-ui/icons/House";
import Chip from "@material-ui/core/Chip";
import IconButton from "@material-ui/core/IconButton";

//MaterialIcon
import BookIcon from "@material-ui/icons/Book";

//MaterialUI Sytle
import { useTheme, StylesProvider } from "@material-ui/core/styles";

//mattermost redux
import { getCurrentTeam } from "mattermost-redux/selectors/entities/teams";

// const { formatText, messageHtmlToComponent } = window.PostUtils;

const TEXT: Record<string, string> = {
    LABEL_BOOKID: "ID",
    LABEL_BOOKNAME: "图书名",
};

function BookPrivateType(props: any) {
    const post = { ...props.post };
    const message = post.message || "";
    const history = useHistory();
    const defaultTheme = useTheme();

    //Current Team
    const currentTeam = useSelector(getCurrentTeam);

    let bookPrivate: BookPrivate;
    try {
        bookPrivate = JSON.parse(message);
    } catch (error) {
        // const formattedText = messageHtmlToComponent(formatText(message));
        // return <div> {formattedText} </div>;
        return <div>{message}</div>;
    }

    const StyledBookPrivate = styled(Paper)(() => {
        const theme = defaultTheme;
        return {
            padding: theme.spacing(2),
            margin: "auto",
            maxWidth: "100%",
            position: "relative",
            "& label,input": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "1rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "1.5rem",
                },
            },
            "& .PaticipantCommon": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "0.8rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "0.8rem",
                },
            },

            "& .PaticipantKeeper": {
                backgroundColor: "teal",
            },
        };
    });

    const handleRoleUserClick = (userName: string) => {
        return () => history.push(`/${currentTeam.name}/messages/@${userName}`);
    };
    const handleLinkToBook = () =>
        history.push(
            `/${currentTeam.name}/pl/${bookPrivate.relations_pri.public}`
        );
    return (
        <StylesProvider injectFirst>
            <StyledBookPrivate>
                <Grid container spacing={2} alignItems={"flex-end"}>
                    <Grid item>
                        <TextField
                            label={TEXT["LABEL_BOOKID"]}
                            value={bookPrivate.id_pri}
                            InputProps={{
                                readOnly: true,
                            }}
                        />
                    </Grid>
                    <Grid item>
                        <TextField
                            label={TEXT["LABEL_BOOKNAME"]}
                            value={bookPrivate.name_pri}
                            InputProps={{
                                readOnly: true,
                            }}
                        />
                    </Grid>
                    <Grid item>
                        <IconButton onClick={handleLinkToBook}>
                            <BookIcon />
                        </IconButton>
                    </Grid>
                    {bookPrivate.keeper_names.map((name, i) => (
                        <Grid item>
                            <Chip
                                size={"medium"}
                                icon={<HouseIcon />}
                                color="primary"
                                label={name}
                                className={"PaticipantCommon PaticipantKeeper"}
                                clickable
                                onClick={handleRoleUserClick(
                                    bookPrivate.keeper_users[i]
                                )}
                            />
                        </Grid>
                    ))}
                </Grid>
            </StyledBookPrivate>
        </StylesProvider>
    );
}

export default React.memo(
    BookPrivateType,
    (prev, next) => prev.post.message === next.post.message
);
