import React from "react";
import { useHistory } from "react-router-dom";
import styled from "@emotion/styled";
import { useSelector } from "react-redux";

import { useTheme, StylesProvider } from "@material-ui/core/styles";
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import TextField from "@material-ui/core/TextField";

import IconButton from "@material-ui/core/IconButton";

//MaterialIcon
import BookIcon from "@material-ui/icons/Book";

//mattermost redux
import { getCurrentTeam } from "mattermost-redux/selectors/entities/teams";
// const { formatText, messageHtmlToComponent } = window.PostUtils;

const TEXT: Record<string, string> = {
    LABEL_BOOKID: "ID",
    LABEL_BOOKNAME: "图书名",
    LABEL_STOCK: "库存",
    LABEL_TRANSMIT_OUT: "正在转出",
    LABEL_LENDING: "正在借阅",
    LABEL_TRANSMIT_IN: "正在转入",
    LABEL_PUBLIC_LINK: "图书链接",
};

function BookInventoryType(props: any) {
    const post = { ...props.post };
    const message = post.message || "";
    const history = useHistory();
    const defaultTheme = useTheme();

    //Current Team
    const currentTeam = useSelector(getCurrentTeam);
    let bookInventory: BookInventory;
    try {
        bookInventory = JSON.parse(message);
    } catch (error) {
        // const formattedText = messageHtmlToComponent(formatText(message));
        // return <div> {formattedText} </div>;
        return <div> {message} </div>;
    }

    const handleLinkToBook = () =>
        history.push(
            `/${currentTeam.name}/pl/${bookInventory.relations_inv.public}`
        );

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
        };
    });
    return (
        <StylesProvider injectFirst>
            <StyledBookPrivate>
                <Grid container spacing={2}>
                    <Grid item>
                        <TextField
                            label={TEXT["LABEL_BOOKID"]}
                            value={bookInventory.id_inv}
                            InputProps={{
                                readOnly: true,
                            }}
                        />
                    </Grid>
                    <Grid item>
                        <TextField
                            label={TEXT["LABEL_BOOKNAME"]}
                            value={bookInventory.name_inv}
                            InputProps={{
                                readOnly: true,
                            }}
                        />
                    </Grid>
                    <Grid item>
                        <TextField
                            label={TEXT["LABEL_STOCK"]}
                            value={bookInventory.stock}
                            InputProps={{
                                readOnly: true,
                            }}
                        />
                    </Grid>
                    <Grid item>
                        <TextField
                            label={TEXT["LABEL_TRANSMIT_OUT"]}
                            value={bookInventory.transmit_out}
                            InputProps={{
                                readOnly: true,
                            }}
                        />
                    </Grid>
                    <Grid item>
                        <TextField
                            label={TEXT["LABEL_LENDING"]}
                            value={bookInventory.lending}
                            InputProps={{
                                readOnly: true,
                            }}
                        />
                    </Grid>
                    <Grid item>
                        <TextField
                            label={TEXT["LABEL_TRANSMIT_IN"]}
                            value={bookInventory.transmit_in}
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
                </Grid>
            </StyledBookPrivate>
        </StylesProvider>
    );
}

export default React.memo(
    BookInventoryType,
    (prev, next) => prev.post.message === next.post.message
);
