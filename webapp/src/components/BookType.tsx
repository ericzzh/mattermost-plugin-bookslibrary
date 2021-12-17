import React from "react";
import { useHistory } from "react-router-dom";
import PropTypes from "prop-types";
import styled from "@emotion/styled";

// import {styled}  from "@mui/system";
// import { makeStyles } from "@material-ui/core/styles";
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import Typography from "@material-ui/core/Typography";
import Breadcrumbs from "@material-ui/core/Breadcrumbs";
import Link from "@material-ui/core/Link";
import Fab from "@material-ui/core/Fab";
import BorrowIcon from "@material-ui/icons/MenuBook";
import WorkerIcon from "@material-ui/icons/PermContactCalendar";
import { useDispatch } from "react-redux";
import { searchForTerm } from "actions/post_action";
import { Client4 } from "mattermost-redux/client";
import manifest from "../manifest";
import { getCurrentUser } from "mattermost-redux/selectors/entities/common";
import { getChannel } from "mattermost-redux/selectors/entities/channels";
import { getCurrentTeam } from "mattermost-redux/selectors/entities/teams";
import { getConfig } from "mattermost-redux/selectors/entities/general";
import { useSelector } from "react-redux";
import Accordion from "@material-ui/core/Accordion";
import AccordionSummary from "@material-ui/core/AccordionSummary";
import AccordionDetails from "@material-ui/core/AccordionDetails";
import ExpandMoreIcon from "@material-ui/icons/ExpandMore";
import Switch from "@material-ui/core/Switch";
import Chip from "@material-ui/core/Chip";
import Button from "@material-ui/core/Button";
import ButtonGroup from "@material-ui/core/ButtonGroup";
import useMediaQuery from "@material-ui/core/useMediaQuery";
import { useTheme, StylesProvider } from "@material-ui/core/styles";
// import { IntlProvider, FormattedMessage, FormattedNumber } from "react-intl";
import { GlobalState } from "mattermost-redux/types/store";
import { Channel } from "mattermost-redux/types/channels";
import InProgress from "./InProgress";
import MsgBox, { MsgBoxProps, useMessageUtils } from "./MsgBox";
import Image from "./Image";

const TEXT: Record<string, string> = {
    BOOK_INDEX_TITLE: "目录",
    BOOK_INTRO_TITLE: "简介",
    NOTHING: "暂无",
    BTN_UPL_PIC_TITLE: "图片",
    BTN_UPL_INTRO_TITLE: "简介",
    BTN_UPL_INDEX_TITLE: "目录",
    TOGGLE_ALLOWED_SUCC: "设置成功",
    TOGGLE_ALLOWED_ERROR: "设置失败,错误:",
    BORROW_SUCC: "请求成功",
    BORROW_ERROR: "请求失败,错误:",
    CONFIRM_BORROW: "借阅:",
    CONFIRM_TOGGLE_BORROW: "转换可借阅状态:",
    LINK_TO_PRI: "非公开",
    LINK_TO_INV: "库存",
    INPUT_REASON: "请输入不可借的理由",
    EMPTY_REASON_ERR: "必须输入不可借理由",
};

const { formatText, messageHtmlToComponent } = window.PostUtils;

BookType.propTypes = {
    post: PropTypes.object.isRequired,
    theme: PropTypes.object.isRequired,
};

function BookType(props: any) {
    const post = { ...props.post };
    const message = post.message || "";
    const dispatch = useDispatch();
    const currentUser = useSelector(getCurrentUser);
    const config = useSelector(getConfig);
    const currentChannel = useSelector<GlobalState, Channel>((state) =>
        getChannel(state, post.channel_id)
    );
    const currentTeam = useSelector(getCurrentTeam);
    const defaultTheme = useTheme();
    const matchesSm = useMediaQuery(defaultTheme.breakpoints.up("sm"));
    // const matchesXs = useMediaQuery(theme.breakpoints.up("xs"));
    const history = useHistory();
    //hooks must be placed before parse book JSON
    const [loading, setLoading] = React.useState(false);
    const mutil = useMessageUtils();

    const postMessage = React.useRef(post.message);
    if (postMessage.current !== post.message) {
        mutil.checkAndDisplayAsyncMsg();
        postMessage.current = post.message;
    }

    const imgSrc = React.useRef("");
    const [imgSrcReplace, setImgSrcReplace] = React.useState("");

    // const [imgSrc, setImgSrc] = React.useState(
    //     `${config.SiteURL}/plugins/${manifest.id}/public/info/${book.id_pub}/cover.jpeg`
    // );
    //

    let book: Book;

    try {
        book = JSON.parse(message);
    } catch (error) {
        // const formattedText = messageHtmlToComponent(formatText(message));
        // return <div> {formattedText} </div>;
        return <div> {message} </div>;
    }

    imgSrc.current = `${config.SiteURL}/plugins/${manifest.id}/public/info/${book.id_pub}/cover.jpeg`;
    const defaultImgSrc = `${config.SiteURL}/plugins/${manifest.id}/public/noImage.png`;

    // It's enough to just send back a post id.
    const handleBorrow = async () => {
        if (!confirm(TEXT["CONFIRM_BORROW"] + book.name_pub)) {
            return;
        }

        const request: BorrowRequestKey = {
            book_post_id: props.post.id,
            borrower_user: currentUser.username,
        };
        setLoading(true);
        const data = await Client4.doFetch<Result>(
            `/plugins/${manifest.id}/borrow`,
            {
                method: "POST",
                body: JSON.stringify(request),
            }
        );
        setLoading(false);

        if (data.error) {
            mutil.setMsgBox({
                open: true,
                text: TEXT["BORROW_ERROR"] + data.error,
                serverity: "error",
            });
            console.error(data);
            return;
        }

        //This message doesn't need to justify the message,
        //because there not modification for book post
        mutil.setMsgBox({
            open: true,
            text: TEXT["BORROW_SUCC"],
            serverity: "success",
        });
    };

    const handleToggleAllowed = async (e: any) => {
        if (!confirm(TEXT["CONFIRM_TOGGLE_BORROW"] + book.name_pub)) {
            return;
        }
        book.isAllowedToBorrow = e.target.checked;
        if (!book.isAllowedToBorrow) {
            const reason = prompt(TEXT["INPUT_REASON"]);
            if (!reason) {
                mutil.setMsgBox({
                    open: true,
                    text: TEXT["EMPTY_REASON_ERR"],
                    serverity: "error",
                });
                return;
            }
            book.reason_of_disallowed = reason;
        }
        book.post_id = props.post.id;
        book.upd_isAllowedToBorrow = true;
        const books = [book];
        const body = JSON.stringify(books);
        const request: BooksRequest = {
            action: "UPLOAD",
            act_user: currentUser.username,
            body: body,
        };

        try {
            setLoading(true);
            mutil.initAsyncMsg(2, {
                open: true,
                text: TEXT["TOGGLE_ALLOWED_SUCC"],
                serverity: "success",
            });
            const data = await Client4.doFetch<Result>(
                `/plugins/${manifest.id}/books`,
                {
                    method: "POST",
                    body: JSON.stringify(request),
                }
            );
            setLoading(false);
            if (data.error) {
                mutil.setMsgBox({
                    open: true,
                    text: TEXT["TOGGLE_ALLOWED_ERROR"] + data.error,
                    serverity: "error",
                });
                console.error(data);
                mutil.disableAsyncMsg();
                return;
            }
        } catch (e) {
            setLoading(false);

            mutil.setMsgBox({
                open: true,
                text: TEXT["TOGGLE_ALLOWED_ERROR"] + e,
                serverity: "error",
            });
            console.error(e);
            mutil.disableAsyncMsg();
            return;
        }
        mutil.checkAndDisplayAsyncMsg();
    };

    const StyledImgWrapper = styled(Grid)(() => {
        const theme = defaultTheme;
        return {
            [theme.breakpoints.up("xs")]: {
                width: 125 * 0.8,
                height: 160 * 0.8,
            },
            [theme.breakpoints.up("sm")]: {
                width: 125 * 1,
                height: 160 * 1,
            },
            "& img": {
                maxWidth: "100%",
                maxHeight: "100%",
                float: "left",
            },
        };
    });

    const StyledBookMainInfo = styled(Grid)(() => {
        const theme = defaultTheme;
        return {
            "& .BookBreadcrumb": {
                "& li": {
                    [theme.breakpoints.up("xs")]: {
                        fontSize: "0.8rem",
                    },
                    [theme.breakpoints.up("sm")]: {
                        fontSize: "1.2rem",
                    },
                },
                "& ol": {
                    paddingLeft: "0 !important",
                },
            },
            "& .BookName": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "2rem",
                    fontWeight: "bold",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "3rem",
                    fontWeight: "bold",
                },
            },
            "&  .EnglishName": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "1rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "1rem",
                },
            },
            "& .AuthorName": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "1.2rem",
                    fontWeight: "bold",
                    marginTop: "0.5rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "1.5rem",
                    fontWeight: "bold",
                    marginTop: "0.5rem",
                },
            },
            "& .BookAttribute": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "0.8rem",
                    marginTop: "-1.5rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "1rem",
                },
            },
        };
    });
    const bookBreadcrumb = (
        <>
            <Breadcrumbs className={"BookBreadcrumb"}>
                <Link
                    color="inherit"
                    onClick={() =>
                        dispatch(
                            searchForTerm(
                                "#c1_" +
                                    book.category1 +
                                    " in:" +
                                    currentChannel.name
                            )
                        )
                    }
                >
                    {book.category1}
                </Link>
                <Link
                    color="inherit"
                    onClick={() =>
                        dispatch(
                            searchForTerm(
                                "#c2_" +
                                    book.category2 +
                                    " in:" +
                                    currentChannel.name
                            )
                        )
                    }
                >
                    {book.category2}
                </Link>
                {book.category3 && (
                    <Link
                        color="inherit"
                        onClick={() =>
                            dispatch(
                                searchForTerm(
                                    "#c3_" +
                                        book.category3 +
                                        " in:" +
                                        currentChannel.name
                                )
                            )
                        }
                    >
                        {book.category3}
                    </Link>
                )}
            </Breadcrumbs>
        </>
    );
    const bookName = (
        <>
            <div className={"BookName"}>{book.name_pub}</div>
            <div className={"EnglishName"}>{book.name_en}</div>
            {matchesSm ? <br /> : <div />}
        </>
    );

    const author = (
        <>
            <div className={"AuthorName"}>{book.author}</div>
            <div className={"EnglishName"}>{book.author_en}</div>
            <br />
        </>
    );
    const publisher = (
        <>
            <div className={"BookAttribute"}>{book.publisher}</div>
            <div className={"EnglishName"}>{book.publisher_en}</div>
        </>
    );
    const pulishDate = (
        <>
            <div className={"BookAttribute"}>{book.publish_date}</div>
        </>
    );
    const translator = (
        <>
            <div className={"BookAttribute"}>{book.translator}</div>
        </>
    );

    const bookAttribute = (
        <Grid container direction={"column"} spacing={2}>
            <Grid item>{author}</Grid>
            <Grid item container spacing={1}>
                <Grid item>{pulishDate}</Grid>
                <Grid item>{publisher}</Grid>
                <Grid item>{translator}</Grid>
            </Grid>
        </Grid>
    );

    const handleImgError = () => {
        setImgSrcReplace(defaultImgSrc);
    };

    const bookMain = (
        <>
            <Grid container spacing={1}>
                <StyledImgWrapper item xs={matchesSm ? 2 : 4}>
                    <Image
                        src={imgSrcReplace || imgSrc.current}
                        handleError={handleImgError}
                    />
                </StyledImgWrapper>
                <StyledBookMainInfo item xs={matchesSm ? 10 : 8}>
                    <Grid container direction={"column"}>
                        <Grid item> {bookBreadcrumb} </Grid>
                        <Grid item>{bookName}</Grid>
                        <Grid item> {bookAttribute} </Grid>
                    </Grid>
                </StyledBookMainInfo>
            </Grid>
        </>
    );

    const StyledBookinfo = styled(Grid)(() => {
        const theme = defaultTheme;
        return {
            "& .BookIntro": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "1rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "1.5rem",
                },
            },
            "& .BookIndex": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "1rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "1.5rem",
                },
            },
            "& .BookAccordion": {
                width: "100%",
            },
        };
    });

    const bookIndex = (
        <Grid container>
            <Accordion className={"BookAccordion"}>
                <AccordionSummary
                    expandIcon={<ExpandMoreIcon />}
                    id="bookIndex-header"
                >
                    <div>{TEXT["BOOK_INDEX_TITLE"]}</div>
                </AccordionSummary>
                <AccordionDetails id="bookIndex-detail">
                    {book.book_index ? book.book_index : TEXT["NOTHING"]}
                </AccordionDetails>
            </Accordion>
        </Grid>
    );

    const bookIntro = (
        <Grid container>
            <Accordion className={"BookAccordion"}>
                <AccordionSummary
                    expandIcon={<ExpandMoreIcon />}
                    id="introduction-header"
                >
                    <div>{TEXT["BOOK_INTRO_TITLE"]}</div>
                </AccordionSummary>
                <AccordionDetails id="introduction-detail">
                    {book.introduction ? book.introduction : TEXT["NOTHING"]}
                </AccordionDetails>
            </Accordion>
        </Grid>
    );

    const bookInfo = (
        <StyledBookinfo container spacing={2}>
            <Grid item xs={12}>
                <Typography>
                    <div className={"BookIntro"}>{bookIntro}</div>
                </Typography>
            </Grid>
            <Grid item xs={12}>
                <Typography>
                    <div className={"BookIndex"}>{bookIndex}</div>
                </Typography>
            </Grid>
        </StyledBookinfo>
    );

    const StyledBookState = styled(Grid)(() => {
        const theme = defaultTheme;
        return {
            flexWrap: "nowrap",
            "& .BorButton": {
                width: 50,
                height: 50,
            },
            "& .Libworker": {
                "& .MuiChip-label": {
                    [theme.breakpoints.up("xs")]: {
                        fontSize: "0.8rem",
                    },
                    [theme.breakpoints.up("sm")]: {
                        fontSize: "1rem",
                    },
                },
                "& .MuiChip-colorPrimary": {
                    backgroundColor: "green",
                },
                [theme.breakpoints.up("xs")]: {
                    fontSize: "1rem",
                    fontWeight: "bold",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "1.5rem",
                    fontWeight: "bold",
                },
            },
        };
    });
    const handleLibworkerClick = (i: number) => {
        return () =>
            history.push(
                `/${currentTeam.name}/messages/@${book.libworker_users[i]}`
            );
    };
    const borrowSection = (
        <StyledBookState
            container
            alignItems={"flex-end"}
            spacing={2}
            justifyContent={"flex-end"}
        >
            <Grid item>
                <Grid container spacing={1} justifyContent={"flex-end"}>
                    {book.libworker_names &&
                        book.libworker_names.map((worker, i) => (
                            <>
                                {
                                    <Grid item className={"Libworker"}>
                                        <Chip
                                            color="primary"
                                            size="medium"
                                            label={worker}
                                            icon={<WorkerIcon />}
                                            clickable
                                            onClick={handleLibworkerClick(i)}
                                        />
                                    </Grid>
                                }
                            </>
                        ))}
                </Grid>
            </Grid>
            <Grid item>
                <Grid container justifyContent={"flex-end"}>
                    <Fab
                        color="primary"
                        aria-label="borrow"
                        className={"BorButton"}
                        onClick={handleBorrow}
                        disabled={book.isAllowedToBorrow ? false : true}
                    >
                        <BorrowIcon />
                    </Fab>
                </Grid>
            </Grid>
        </StyledBookState>
    );

    const StyledReasonOfDisallowed = styled(Grid)(() => {
        const theme = defaultTheme;
        return {
            color: "red",
            marginTop: "1rem",
            [theme.breakpoints.up("xs")]: {
                fontSize: "1.2rem",
            },
            [theme.breakpoints.up("sm")]: {
                fontSize: "1.5rem",
            },
        };
    });

    const ReasonOfDisallowed = book.reason_of_disallowed && (
        <StyledReasonOfDisallowed
            container
            justifyContent={"flex-end"}
            alignItems={"flex-end"}
        >
            <Grid item>{book.reason_of_disallowed}</Grid>
        </StyledReasonOfDisallowed>
    );

    const StyledBorrowSection = styled(Grid)({
        height: "100%",
    });

    const main = (
        <Grid container spacing={2}>
            <Grid item xs={12} sm={8}>
                {bookMain}
            </Grid>
            <Grid item xs={12} sm={4}>
                <StyledBorrowSection
                    container
                    direction={"column"}
                    justifyContent={"flex-end"}
                >
                    {/*
                    //Next Phase
                    <Grid item>{bookInfo}</Grid>
                    */}
                    <Grid item>{borrowSection}</Grid>
                    {book.reason_of_disallowed && (
                        <Grid item>{ReasonOfDisallowed}</Grid>
                    )}
                </StyledBorrowSection>
            </Grid>
        </Grid>
    );

    const handleLinkToBookPartPost = (postId: string) =>
        history.push(`/${currentTeam.name}/pl/${postId}`);

    const buttons = (
        <>
            <Grid container justifyContent={"flex-end"} alignItems={"center"}>
                <Grid item>
                    <ButtonGroup size="small">
                        <Button
                            onClick={() =>
                                handleLinkToBookPartPost(
                                    book.relations_pub.inventory
                                )
                            }
                        >
                            {TEXT["LINK_TO_INV"]}
                        </Button>
                        <Button
                            onClick={() =>
                                handleLinkToBookPartPost(
                                    book.relations_pub.private
                                )
                            }
                        >
                            {TEXT["LINK_TO_PRI"]}
                        </Button>
                        {/* Next phase
                        <Button>{TEXT["BTN_UPL_PIC_TITLE"]}</Button>
                        <Button>{TEXT["BTN_UPL_INTRO_TITLE"]}</Button>
                        <Button>{TEXT["BTN_UPL_INDEX_TITLE"]}</Button>
                        */}
                    </ButtonGroup>
                </Grid>
                <Grid item>
                    <Switch
                        checked={book.isAllowedToBorrow}
                        name="isAllowedToBorrow"
                        onChange={handleToggleAllowed}
                    />
                </Grid>
            </Grid>
            <Grid container justifyContent={"flex-end"} alignItems={"center"}>
                <Grid item>{`ID: ${book.id_pub}`}</Grid>
            </Grid>
        </>
    );

    const StyledPaper = styled(Paper)(() => {
        const theme = defaultTheme;
        return {
            padding: theme.spacing(2),
            margin: "auto",
            maxWidth: "100%",
            position: "relative",
        };
    });

    const isLibworker = () => {
        const libworkers = book.libworker_users;
        if (
            libworkers.findIndex((user) => user == currentUser.username) !== -1
        ) {
            return true;
        }
        return false;
    };

    return (
        // <IntlProvider locale="en" defaultLocale="en">
        <StylesProvider injectFirst>
            <StyledPaper>
                <Grid container direction={"column"} spacing={1}>
                    <Grid item>{main}</Grid>
                    {isLibworker() && <Grid item>{buttons}</Grid>}
                </Grid>
                {loading && <InProgress open={loading} />}
                <MsgBox {...mutil.msgBox} close={mutil.onCloseMsg} />
            </StyledPaper>
        </StylesProvider>
        // </IntlProvider>
    );
}

export default React.memo(
    BookType,
    (prev, next) => prev.post.message === next.post.message
);
